package bf

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const maxLoaderTicks = 20

// Start subscribes to Telegram updates and processes them until ctx is cancelled
// or the updates channel is closed. Register all static handlers (commands,
// inline buttons, middlewares) before calling Start — registering after Start
// is allowed but goes through internal locks and is slightly more expensive.
//
// Start always releases its background goroutines on return via Stop, so calling
// Stop manually after Start is safe but unnecessary.
func (b *ChatBotImpl) Start(ctx context.Context) error {
	b.logger.Debugf("starting bot")

	if err := b.validateConfiguration(); err != nil {
		return fmt.Errorf("failed to validate configuration: %w", err)
	}
	if b.tgbot == nil {
		return errors.New("telegram client is nil; check the error returned by NewBot")
	}

	defer b.Stop()

	updates := b.tgbot.GetUpdatesChan(tgbotapi.UpdateConfig{
		Timeout: 60,
	})

	return b.mainLoop(ctx, updates)
}

// GetFileURL resolves a Telegram fileID to a directly downloadable URL.
func (b *ChatBotImpl) GetFileURL(fileID string) (string, error) {
	url, err := b.tgbot.GetFileDirectURL(fileID)
	if err != nil {
		return "", fmt.Errorf("failed to get file direct url: %w", err)
	}

	return url, nil
}

// LoaderButton sends a placeholder message and animates it by editing the
// message text to successive entries of loadScreen, until the returned cancel
// function is called or maxLoaderTicks ticks elapse. Always defer cancel().
//
// The loader goroutine is also tied to the bot's shutdown signal: calling
// Stop on the bot cancels every active loader.
//
// If the initial Send fails, the loader gives up immediately rather than
// editing a non-existent message MessageID=0 in a loop.
func (b *ChatBotImpl) LoaderButton(chatID int64, loadScreen []string) context.CancelFunc {
	if len(loadScreen) == 0 {
		b.logger.Errorf("LoaderButton: loadScreen cannot be empty")
		return func() {}
	}

	loaderCtx, cancel := context.WithCancel(context.Background())

	// Bridge the bot's shutdown channel into the loader's context so Stop
	// terminates loaders without the caller having to hold every cancel.
	go func() {
		select {
		case <-loaderCtx.Done():
		case <-b.shutdown:
			cancel()
		}
	}()

	go func() {
		msg := tgbotapi.NewMessage(chatID, loadScreen[0])

		sentMsg, err := b.tgbot.Send(msg)
		if err != nil {
			b.logger.Errorf("failed to send loader message: %s", err)
			cancel()
			return
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
	ticker := time.NewTicker(loaderTick())

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
			if len(loadScreen) != 1 {
				msg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, loadScreen[count])
				if _, err := b.tgbot.Send(msg); err != nil {
					b.logger.Errorf("failed to send loader message: %s", err)
				}
			}
		}
	}
}

// RegisterIButton attaches an inline-button handler to the default layer.
func (b *ChatBotImpl) RegisterIButton(btn string, handler HandlerFunc) {
	b.defaultLayerMutex.Lock()
	b.defaultHandlerLayer.RegisterIButton(btn, handler)
	b.defaultLayerMutex.Unlock()
}

// SelfUserName returns the bot's own Telegram username.
func (b *ChatBotImpl) SelfUserName() string {
	return b.tgbot.Self().UserName
}

// RegisterButton attaches a reply-keyboard button handler to the default layer.
func (b *ChatBotImpl) RegisterButton(btn string, handler HandlerFunc) {
	b.defaultLayerMutex.Lock()
	b.defaultHandlerLayer.RegisterButton(btn, handler)
	b.defaultLayerMutex.Unlock()
}

// RegisterAudio attaches a voice-message handler to the default layer.
// The Voice payload is exposed via Event.Voice.
func (b *ChatBotImpl) RegisterAudio(handler HandlerFunc) {
	b.defaultLayerMutex.Lock()
	b.defaultHandlerLayer.RegisterVoice(handler)
	b.defaultLayerMutex.Unlock()
}

// SendText sends a plain text message without affecting any chat layer.
// The configured parse mode (WithParseMode) is applied just like in SendMsg.
func (b *ChatBotImpl) SendText(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = b.parseMode
	if _, err := b.tgbot.Send(msg); err != nil {
		return fmt.Errorf("failed to send text: %w", err)
	}
	return nil
}

// NewLayer constructs a fresh layer carrying optional message text.
// The layer is not yet bound to any chat — pass it to SendMsg to install it.
// msgText is joined with a single space; an empty msgText yields an empty text.
func (b *ChatBotImpl) NewLayer(msgText ...any) *HandlerLayer {
	b.logger.Debugf("creating new layer")

	var text string
	if len(msgText) > 0 {
		text = strings.TrimRight(fmt.Sprintln(msgText...), "\n")
	}

	return &HandlerLayer{
		text:                text,
		commandHandler:      make(map[string]CommandHandler),
		textHandler:         make(map[string]TextHandler),
		buttonTextHandler:   make(map[string]TextHandler),
		buttonHandler:       make(map[string]InlineButtonHandler),
		audioHandler:        nil,
		layerDefaultHandler: nil,
		ttl:                 time.Now().Add(b.defaultTTL),
		rowMode:             false,
	}
}

// SendMsg renders the layer (text + buttons), sends it to the chat and
// installs the layer as the next-message expectation for chatID.
// Returns an error if layer is nil.
func (b *ChatBotImpl) SendMsg(chatID int64, layer *HandlerLayer) error {
	if layer == nil {
		return errors.New("SendMsg: layer is nil")
	}

	message := tgbotapi.NewMessage(chatID, layer.text)
	sortedIButtonsSlice := layer.sortedIButtonsSlice()
	rawIButtons := make([]tgbotapi.InlineKeyboardButton, 0, len(sortedIButtonsSlice))

	sortedButtonsSlice := layer.sortedButtonsSlice()
	rawButtons := make([]tgbotapi.KeyboardButton, 0, len(sortedButtonsSlice))

	for _, button := range sortedIButtonsSlice {
		rawIButtons = append(rawIButtons, button.button)
	}

	for _, button := range sortedButtonsSlice {
		rawButtons = append(rawButtons, tgbotapi.NewKeyboardButton(button.text))
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

	message.ParseMode = b.parseMode

	if _, err := b.tgbot.Send(message); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	b.setLayer(layer, chatID)

	return nil
}

// RetryLastLayer re-sends the layer that was active during the previous
// message in the chat. If newText is non-empty it overrides the layer's
// text without mutating the original layer.
func (b *ChatBotImpl) RetryLastLayer(event Event, newText string) error {
	previousLayer := event.lastLayer

	if previousLayer == nil {
		return fmt.Errorf("RetryLastLayer: no previous layer for chat %d", event.ChatID)
	}

	if newText != "" {
		// Copy the layer before mutation: another goroutine may still hold
		// a pointer to the original (e.g. the default layer).
		layerCopy := *previousLayer
		layerCopy.text = newText
		previousLayer = &layerCopy
	}

	return b.SendMsg(event.ChatID, previousLayer)
}

// RegisterCommand attaches a slash-command handler to the default layer.
// command must include the leading slash (e.g. "/start").
func (b *ChatBotImpl) RegisterCommand(command string, handler HandlerFunc) {
	b.defaultLayerMutex.Lock()
	b.defaultHandlerLayer.RegisterCommand(command, handler)
	b.defaultLayerMutex.Unlock()
}

// RegisterErrorHandler installs the function called whenever a handler returns
// an error. Safe to call concurrently with the dispatcher.
func (b *ChatBotImpl) RegisterErrorHandler(handler ErrorHandlerFunc) {
	b.errorHandlerMutex.Lock()
	b.errorHandler = handler
	b.errorHandlerMutex.Unlock()
}

// RegisterMiddleware appends a middleware to the chain applied to every handler.
// Middlewares are applied in registration order (last added runs outermost).
// Safe to call concurrently with the dispatcher.
func (b *ChatBotImpl) RegisterMiddleware(middleware MiddlewareFunc) {
	b.middlewaresMutex.Lock()
	b.middlewares = append(b.middlewares, middleware)
	b.middlewaresMutex.Unlock()
}

// RegisterDefaultHandler sets the fallback handler invoked when no other
// handler in the active layer matches the incoming event. Safe to call
// concurrently with the dispatcher.
func (b *ChatBotImpl) RegisterDefaultHandler(handler HandlerFunc) {
	b.defaultLayerMutex.Lock()
	b.defaultHandlerLayer.layerDefaultHandler = handler
	b.defaultLayerMutex.Unlock()
}

// findAndWipeChatLayerHandler returns the chat-specific layer (consuming it)
// or the default layer when no chat-specific layer is installed.
func (b *ChatBotImpl) findAndWipeChatLayerHandler(chatID int64) *HandlerLayer {
	layer, ok := b.getAndDeleteLayer(chatID)
	if !ok {
		return b.defaultHandlerLayer
	}

	return layer
}
