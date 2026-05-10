package bf

import (
	"errors"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// cleanerTickInterval is the cadence at which the cleaner sweeps expired layers.
// It is a var (not a const) so tests can shorten it without touching production paths.
var cleanerTickInterval = 10 * time.Minute

// getAndDeleteLayer atomically returns and deletes the layer for chatID.
// Combining the two operations under one lock prevents a TOCTOU race
// where two goroutines could read and serve the same layer.
func (b *ChatBotImpl) getAndDeleteLayer(chatID int64) (*HandlerLayer, bool) {
	b.layersMutex.Lock()
	defer b.layersMutex.Unlock()

	layer, ok := b.chatHandlerLayers[chatID]
	if ok {
		delete(b.chatHandlerLayers, chatID)
	}

	return layer, ok
}

// sweepExpiredLayers removes every chat layer whose TTL has elapsed.
// Extracted from cleaner so it can be invoked synchronously from tests.
func (b *ChatBotImpl) sweepExpiredLayers() {
	b.layersMutex.Lock()
	defer b.layersMutex.Unlock()
	for chatID, layer := range b.chatHandlerLayers {
		if layer.IsExpired() {
			delete(b.chatHandlerLayers, chatID)
		}
	}
}

// cleaner periodically removes expired chat layers. Stops when shutdown is closed.
func (b *ChatBotImpl) cleaner() {
	ticker := time.NewTicker(cleanerTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.shutdown:
			return
		case <-ticker.C:
			b.sweepExpiredLayers()
		}
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
	if rowMode && len(rawIButtons) >= 2 {
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
	b.middlewaresMutex.RLock()
	mws := b.middlewares
	b.middlewaresMutex.RUnlock()

	for _, middleware := range mws {
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
