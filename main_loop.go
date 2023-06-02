package bf

import (
	"context"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const chatControllerTTL = 1 * time.Minute

// chatController control that only 1 message from user will be processed at the same time.
type chatController struct {
	userInWork map[int64]time.Time
	mux        *sync.Mutex
}

func (c chatController) cleanOld() {
	for {
		time.Sleep(chatControllerTTL)
		c.mux.Lock()
		for userID, lastTime := range c.userInWork {
			if lastTime.Add(chatControllerTTL).Before(time.Now()) {
				delete(c.userInWork, userID)
			}
		}
		c.mux.Unlock()
	}
}

// LockChat returns true if chat is already in work.
func (c chatController) LockChat(chatID int64) bool {
	c.mux.Lock()
	defer c.mux.Lock()

	_, ok := c.userInWork[chatID]
	if ok {
		return true
	}

	c.userInWork[chatID] = time.Now()

	return false
}

func (c chatController) UnlockChat(chatID int64) {
	c.mux.Lock()
	defer c.mux.Lock()
	delete(c.userInWork, chatID)
}

func newChatController() chatController {
	blocker := chatController{
		userInWork: make(map[int64]time.Time),
	}
	go blocker.cleanOld()

	return blocker
}

func (b *ChatBotImpl) mainLoop(ctx context.Context, updates tgbotapi.UpdatesChannel) error {
	control := newChatController()

	for update := range updates {
		go func(update tgbotapi.Update) {
			event, ok := newEvent(update)
			if !ok {
				b.errorHandler(ctx, event, ErrUnparsedEvent)
				return
			}

			b.logger.Debugf("got event: %#v", event)
			// chat locked for user messages until handler is finished. Every message will be skipped.
			skip := control.LockChat(event.ChatID)
			if skip {
				b.logger.Debugf("skip event: %#v", event)
				return
			}

			defer control.UnlockChat(event.ChatID)

			layer := b.findAndWipeChatLayerHandler(event.ChatID)
			b.logger.Debugf("got layer: %#v", layer)
			event.lastLayer = layer
			handlerFunc := b.availableHandlerFromLayers(event, layer, b.defaultHandlerLayer)

			err := b.applyMiddlewares(handlerFunc)(ctx, event)
			if err != nil {
				b.errorHandler(ctx, event, err)
			}
		}(update)
	}

	return nil
}
