package bf

import (
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/pkg/errors"
)

func (b *ChatBotImpl) deleteLayer(chatID int64) {
	b.layersMutex.Lock()
	delete(b.chatHandlerLayers, chatID)
	b.layersMutex.Unlock()
}

func (b *ChatBotImpl) getLayer(chatID int64) (*HandlerLayer, bool) {
	b.layersMutex.RLock()
	defer b.layersMutex.RUnlock()

	layer, ok := b.chatHandlerLayers[chatID]

	return layer, ok
}

// cleaner removes expired layers left by users.
func (b *ChatBotImpl) cleaner() {
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

func (b *ChatBotImpl) validateConfiguration() error {
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

func (b *ChatBotImpl) buildInlineKeyboard(
	rawIButtons []tgbotapi.InlineKeyboardButton,
	rowMode bool,
) tgbotapi.InlineKeyboardMarkup {
	if rowMode {
		return tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
				rawIButtons[:len(rawIButtons)-1],
				{rawIButtons[len(rawIButtons)-1]},
			},
		}
	}

	iButtons := make([][]tgbotapi.InlineKeyboardButton, 0, len(rawIButtons))
	for _, button := range rawIButtons {
		iButtons = append(iButtons, []tgbotapi.InlineKeyboardButton{button})
	}

	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: iButtons}
}

func (b *ChatBotImpl) applyMiddlewares(handlerFunc HandlerFunc) HandlerFunc {
	for _, middleware := range b.middlewares {
		handlerFunc = middleware(handlerFunc)
	}

	return handlerFunc
}

func (b *ChatBotImpl) availableHandlerFromLayers(event Event, chatLayer, defaultLayer *HandlerLayer) HandlerFunc {
	handlerFunc := chatLayer.Handler(event)
	if handlerFunc == nil {
		handlerFunc = defaultLayer.Handler(event)
	}

	return handlerFunc
}

func (b *ChatBotImpl) setLayer(layer *HandlerLayer, chatID int64) {
	b.layersMutex.Lock()
	b.chatHandlerLayers[chatID] = layer
	b.layersMutex.Unlock()
}
