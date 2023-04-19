package bf

import (
	"context"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var ErrUnparsedEvent = errors.New("unparsed event")

var _ BotBuilder = &botBuilder{}

type botBuilder struct {
	tgbot               *tgbotapi.BotAPI
	chatHandlerLayers   map[int64]*HandlerLayer
	retryLayers         map[int64]*string
	defaultHandlerLayer *HandlerLayer
	layersMutex         sync.RWMutex
	retryLayersMutex    sync.RWMutex
	middlewares         []MiddlewareFunc
	errorHandler        ErrorHandlerFunc
	debug               bool
}

func (b *botBuilder) RegisterIButton(btn string, handler HandlerFunc) {
	b.defaultHandlerLayer.RegisterIButton(btn, handler)
}

func (b *botBuilder) SelfUserName() string {
	return b.tgbot.Self.UserName
}

func (b *botBuilder) RegisterButton(btn string, handler HandlerFunc) {
	b.defaultHandlerLayer.RegisterButton(btn, handler)
}

func (b *botBuilder) SendText(chatID int64, text string) error {
	logrus.Debugf("sending text to chat %d: %s", chatID, text)
	layer := b.NewLayer()
	layer.AddText(text)
	_, err := b.tgbot.Send(tgbotapi.NewMessage(chatID, layer.text))
	return err
}

func (b *botBuilder) NewLayer() *HandlerLayer {
	logrus.Debugf("creating new layer")
	return &HandlerLayer{
		commandHandler:     make(map[string]CommandHandler),
		textHandler:        make(map[string]TextHandler),
		buttonHandler:      make(map[string]InlineButtonHandler),
		generalMiddlewares: b.middlewares,
		ttl:                time.Now().Add(time.Hour * 24), // we don't want to expire it
	}
}

func NewBot(apikey string, debug bool) (BotBuilder, error) {
	logrus.Debugf("creating new bot")
	bot, err := tgbotapi.NewBotAPI(apikey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create bot")
	}

	b := &botBuilder{
		tgbot:             bot,
		chatHandlerLayers: make(map[int64]*HandlerLayer),
		retryLayers:       map[int64]*string{},
		layersMutex:       sync.RWMutex{},
		retryLayersMutex:  sync.RWMutex{},
		middlewares:       make([]MiddlewareFunc, 0),
		errorHandler:      nil,
		debug:             debug,
	}
	b.defaultHandlerLayer = b.NewLayer()
	b.defaultHandlerLayer.ttl = time.Now().Add(time.Hour * 24 * 365 * 100) // we don't want to expire it
	b.RegisterErrorHandler(b.defaultErrorHandler)
	b.RegisterDefaultHandler(b.defaultEventHandler)
	go b.cleaner()
	return b, nil
}

func (b *botBuilder) Start(ctx context.Context) error {
	logrus.Debugf("starting bot")
	err := b.validateConfiguration()
	if err != nil {
		return errors.Wrap(err, "failed to validate configuration")
	}
	updates := b.tgbot.GetUpdatesChan(tgbotapi.UpdateConfig{
		Timeout: 60,
	})

	return b.mainLoop(ctx, updates)
}

func (b *botBuilder) mainLoop(ctx context.Context, updates tgbotapi.UpdatesChannel) error {
	for update := range updates {
		event, ok := b.ParseEvent(update)
		if !ok {
			b.errorHandler(context.Background(), event, ErrUnparsedEvent)
			continue
		}
		logrus.Debugf("got event: %#v", event)
		layer := b.findAndWipeChatLayerHandler(event.ChatID)
		event.lastLayer = layer
		handlerFunc := b.availableHandlerFromLayers(event, layer, b.defaultHandlerLayer)
		err := b.applyMiddlewares(handlerFunc)(ctx, event)
		if err != nil {
			b.errorHandler(ctx, event, err)
		}
	}
	return nil
}

func (b *botBuilder) setLayer(layer *HandlerLayer, chatID int64) {
	b.layersMutex.Lock()
	b.chatHandlerLayers[chatID] = layer
	b.layersMutex.Unlock()
}

func (b *botBuilder) applyMiddlewares(handlerFunc HandlerFunc) HandlerFunc {
	for _, middleware := range b.middlewares {
		handlerFunc = middleware(handlerFunc)
	}
	return handlerFunc
}

func (b *botBuilder) availableHandlerFromLayers(event Event, chatLayer, defaultLayer *HandlerLayer) HandlerFunc {
	handlerFunc := chatLayer.Handler(event)
	if handlerFunc == nil {
		handlerFunc = defaultLayer.Handler(event)
	}
	return handlerFunc
}

func (b *botBuilder) SendMsg(chatID int64, layer *HandlerLayer) error {
	message := tgbotapi.NewMessage(chatID, layer.text)
	var rawIButtons []tgbotapi.InlineKeyboardButton
	var rawButtons []tgbotapi.KeyboardButton

	for _, button := range layer.sortedIButtonsSlice() {
		rawIButtons = append(rawIButtons, button.button)
	}

	for _, button := range layer.textHandler {
		if button.kind == TextHandlerKindButton {
			rawButtons = append(rawButtons, tgbotapi.NewKeyboardButton(button.text))
		}
	}

	isInline := len(rawIButtons) > 0
	if isInline {
		message.ReplyMarkup = b.buildInlineKeyboard(rawIButtons)
	}
	isRegular := len(rawButtons) > 0
	if isRegular {
		message.ReplyMarkup = tgbotapi.NewReplyKeyboard(rawButtons)
	}
	if isInline && isRegular {
		return errors.New("can't send both inline and regular buttons")
	}

	b.setLayer(layer, chatID)

	_, err := b.tgbot.Send(message)
	return errors.Wrap(err, "failed to send message")
}

func (b *botBuilder) buildInlineKeyboard(rawIButtons []tgbotapi.InlineKeyboardButton) tgbotapi.InlineKeyboardMarkup {

	var iButtons [][]tgbotapi.InlineKeyboardButton
	for _, button := range rawIButtons {
		iButtons = append(iButtons, []tgbotapi.InlineKeyboardButton{button})
	}

	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: iButtons}
}

func (b *botBuilder) RetryLastLayer(event Event, newText string) error {
	previousLayer := event.lastLayer
	if previousLayer == nil {
		return errors.Errorf("RetryLastLayer: no previous layer for chat %d", event.ChatID)
	}
	if newText != "" {
		previousLayer.text = newText
	}

	return b.SendMsg(event.ChatID, previousLayer)
}

func (b *botBuilder) RegisterCommand(command string, handler HandlerFunc) {
	b.defaultHandlerLayer.RegisterCommand(command, handler)
}

func (b *botBuilder) RegisterErrorHandler(handler ErrorHandlerFunc) {
	b.errorHandler = handler
}

func (b *botBuilder) RegisterMiddleware(middleware MiddlewareFunc) {
	b.middlewares = append(b.middlewares, middleware)
}

func (b *botBuilder) findAndWipeChatLayerHandler(chatID int64) *HandlerLayer {
	layer, ok := b.getLayer(chatID)
	if !ok {
		return b.defaultHandlerLayer
	}
	if ok {
		b.deleteLayer(chatID)
	}
	return layer
}

func (b *botBuilder) deleteLayer(chatID int64) {
	b.layersMutex.Lock()
	delete(b.chatHandlerLayers, chatID)
	b.layersMutex.Unlock()
}

func (b *botBuilder) getLayer(chatID int64) (*HandlerLayer, bool) {
	b.layersMutex.RLock()
	defer b.layersMutex.RUnlock()
	layer, ok := b.chatHandlerLayers[chatID]
	return layer, ok
}

func (b *botBuilder) cleaner() {
	for {
		time.Sleep(10 * time.Minute)
		b.layersMutex.Lock()
		for chatID, layer := range b.chatHandlerLayers {
			if layer.IsExpired() {
				delete(b.chatHandlerLayers, chatID)
			}
		}
		b.layersMutex.Unlock()
	}
}

func (b *botBuilder) RegisterDefaultHandler(handler HandlerFunc) {
	b.defaultHandlerLayer.layerDefaultHandler = handler
}

func (b *botBuilder) validateConfiguration() error {
	if b.errorHandler == nil {
		return errors.New("error handler is not set")
	}
	if b.defaultHandlerLayer == nil {
		return errors.New("default handler layer is not set")
	}
	if b.defaultHandlerLayer.layerDefaultHandler == nil {
		return errors.New("default handler is not set")
	}
	return nil
}
