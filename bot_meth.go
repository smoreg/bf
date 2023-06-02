package bf

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/pkg/errors"
)

const maxLoaderTicks = 20

// Start starts bot.
func (b *ChatBotImpl) Start(ctx context.Context) error {
	b.logger.Debugf("starting bot")

	err := b.validateConfiguration()
	if err != nil {
		return errors.Wrap(err, "failed to validate configuration")
	}

	updates := b.tgbot.GetUpdatesChan(tgbotapi.UpdateConfig{
		Timeout: 60,
	})

	return b.mainLoop(ctx, updates)
}

// GetFileURL returns direct url to telegram file.
func (b *ChatBotImpl) GetFileURL(fileID string) (string, error) {
	url, err := b.tgbot.GetFileDirectURL(fileID)
	if err != nil {
		return "", errors.Wrap(err, "failed to get file direct url")
	}

	return url, nil
}

// LoaderButton creates loader button.
// Loader button is a button that will be shown while some long operation is in progress by editing last message with
// texts from loadScreen.
// !!! WARNING !!!
// Don't forget to call cancel() when operation is finished.
func (b *ChatBotImpl) LoaderButton(chatID int64, loadScreen []string) context.CancelFunc {
	loaderCtx, cancel := context.WithCancel(context.Background())

	go func() {
		msg := tgbotapi.NewMessage(chatID, loadScreen[0])

		sentMsg, err := b.tgbot.Send(msg)
		if err != nil {
			b.logger.Errorf("failed to send loader message: %s", err)
		}

		b.loaderButtonLoop(loaderCtx, chatID, sentMsg, cancel, loadScreen)
	}()

	return cancel
}

func (b *ChatBotImpl) loaderButtonLoop(ctx context.Context, chatID int64, sentMsg tgbotapi.Message,
	cancel context.CancelFunc,
	loadScreen []string,
) {
	count := 0
	fullCount := 0
	ticker := time.NewTicker(loaderTickDelay * time.Millisecond)

	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count++
			fullCount++

			if fullCount > maxLoaderTicks {
				if b.debug {
					msg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, "Load screen timeout")
					if _, err := b.tgbot.Send(msg); err != nil {
						b.logger.Errorf("failed to send loader message: %s", err)
					}
				}

				cancel()

				return
			}

			count %= len(loadScreen)
			if len(loadScreen) != 1 { // don't edit message if there is only one screen
				msg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, loadScreen[count])
				if _, err := b.tgbot.Send(msg); err != nil {
					b.logger.Errorf("failed to send loader message: %s", err)
				}
			}
		}
	}
}

// RegisterIButton registers a new inline button with the given text and handler function.
// btn - text to be displayed on the button.
// handler - function to be called when the button is pressed.
func (b *ChatBotImpl) RegisterIButton(btn string, handler HandlerFunc) {
	b.defaultHandlerLayer.RegisterIButton(btn, handler)
}

// SelfUserName returns bot username.
func (b *ChatBotImpl) SelfUserName() string {
	return b.tgbot.Self.UserName
}

// RegisterButton registers a new button with the given text and handler function.
func (b *ChatBotImpl) RegisterButton(btn string, handler HandlerFunc) {
	b.defaultHandlerLayer.RegisterButton(btn, handler)
}

// RegisterAudio registers a handler for Voice msg. Voice will be added as file to Event.
func (b *ChatBotImpl) RegisterAudio(handler HandlerFunc) {
	b.defaultHandlerLayer.RegisterVoice(handler)
}

// SendText sends simple text to chat. Doesn't affect any layers.
func (b *ChatBotImpl) SendText(chatID int64, text string) error {
	_, err := b.tgbot.Send(tgbotapi.NewMessage(chatID, text))
	return errors.Wrap(err, "failed to send text")
}

// NewLayer creates new layer. Layer is a set of handlers for different types of events.
// msgText - text that will be sent to user when layer is activated via `SendMsg` method.
func (b *ChatBotImpl) NewLayer(msgText ...any) *HandlerLayer {
	b.logger.Debugf("creating new layer")

	return &HandlerLayer{
		text:                fmt.Sprintln(msgText...),
		commandHandler:      make(map[string]CommandHandler),
		textHandler:         make(map[string]TextHandler),
		buttonHandler:       make(map[string]InlineButtonHandler),
		audioHandler:        nil,
		layerDefaultHandler: nil,
		ttl:                 time.Now().Add(time.Hour * 24),
		generalMiddlewares:  b.middlewares,
		rowMode:             false,
	}
}

// SendMsg sends layer to chat with given ID. Text and buttons will be sent, and layer will be set as current layer
// for chatID.
func (b *ChatBotImpl) SendMsg(chatID int64, layer *HandlerLayer) error {
	message := tgbotapi.NewMessage(chatID, layer.text)
	sortedIButtonsSlice := layer.sortedIButtonsSlice()
	rawIButtons := make([]tgbotapi.InlineKeyboardButton, 0, len(sortedIButtonsSlice))

	var rawButtons []tgbotapi.KeyboardButton

	for _, button := range sortedIButtonsSlice {
		rawIButtons = append(rawIButtons, button.button)
	}

	for _, button := range layer.textHandler {
		if button.kind == TextHandlerKindButton {
			rawButtons = append(rawButtons, tgbotapi.NewKeyboardButton(button.text))
		}
	}

	isInline := len(rawIButtons) > 0
	if isInline {
		message.ReplyMarkup = b.buildInlineKeyboard(rawIButtons, layer.rowMode)
	}

	isRegular := len(rawButtons) > 0
	if isRegular {
		message.ReplyMarkup = tgbotapi.NewReplyKeyboard(rawButtons)
	}

	if isInline && isRegular {
		return errors.New("can't send both inline and regular buttons")
	}

	b.setLayer(layer, chatID)

	message.ParseMode = b.parseMode

	_, err := b.tgbot.Send(message)

	return errors.Wrap(err, "failed to send message")
}

// RetryLastLayer sends previous layer to chat with given ID. If newText is not empty, it will be used instead of
// previous layer text.
func (b *ChatBotImpl) RetryLastLayer(event Event, newText string) error {
	previousLayer := event.lastLayer

	if previousLayer == nil {
		return errors.Errorf("RetryLastLayer: no previous layer for chat %d", event.ChatID)
	}

	if newText != "" {
		previousLayer.text = newText
	}

	return b.SendMsg(event.ChatID, previousLayer)
}

// RegisterCommand registers a new command with the given name and handler function.
func (b *ChatBotImpl) RegisterCommand(command string, handler HandlerFunc) {
	b.defaultHandlerLayer.RegisterCommand(command, handler)
}

// RegisterErrorHandler registers a new error handler.
func (b *ChatBotImpl) RegisterErrorHandler(handler ErrorHandlerFunc) {
	b.errorHandler = handler
}

// RegisterMiddleware registers a new middleware.
func (b *ChatBotImpl) RegisterMiddleware(middleware MiddlewareFunc) {
	b.middlewares = append(b.middlewares, middleware)
}

// RegisterDefaultHandler registers a new default handler.
func (b *ChatBotImpl) RegisterDefaultHandler(handler HandlerFunc) {
	b.defaultHandlerLayer.layerDefaultHandler = handler
}

// findAndWipeChatLayerHandler finds layer for chatID and deletes it from layers map.
func (b *ChatBotImpl) findAndWipeChatLayerHandler(chatID int64) *HandlerLayer {
	layer, ok := b.getLayer(chatID)

	if !ok {
		return b.defaultHandlerLayer
	}

	if ok {
		b.deleteLayer(chatID)
	}

	return layer
}
