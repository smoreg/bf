package bf

import (
	"context"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// chatControllerTTL is how long a per-chat lock stays alive before being
// reclaimed by the background sweeper. Var so tests can shorten it.
var chatControllerTTL = 1 * time.Minute

// chatController serialises message processing per chat: while one update from
// a given chat is being handled, subsequent updates from the same chat are
// dropped. This prevents handler interleaving for the same user.
type chatController struct {
	userInWork map[int64]time.Time
	mux        *sync.Mutex
}

// cleanOld evicts stale chat locks. Stops when ctx is cancelled.
func (c chatController) cleanOld(ctx context.Context) {
	ticker := time.NewTicker(chatControllerTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mux.Lock()
			for userID, lastTime := range c.userInWork {
				if lastTime.Add(chatControllerTTL).Before(time.Now()) {
					delete(c.userInWork, userID)
				}
			}
			c.mux.Unlock()
		}
	}
}

// LockChat returns true if the chat is already being processed.
// When false, the chat is registered as in-progress and must be released
// with UnlockChat by the caller.
func (c chatController) LockChat(chatID int64) bool {
	c.mux.Lock()
	defer c.mux.Unlock()

	if _, ok := c.userInWork[chatID]; ok {
		return true
	}

	c.userInWork[chatID] = time.Now()

	return false
}

func (c chatController) UnlockChat(chatID int64) {
	c.mux.Lock()
	defer c.mux.Unlock()
	delete(c.userInWork, chatID)
}

func newChatController(ctx context.Context) chatController {
	blocker := chatController{
		userInWork: make(map[int64]time.Time),
		mux:        &sync.Mutex{},
	}
	go blocker.cleanOld(ctx)

	return blocker
}

func (b *ChatBotImpl) mainLoop(ctx context.Context, updates tgbotapi.UpdatesChannel) error {
	control := newChatController(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			go b.handleUpdate(ctx, control, update)
		}
	}
}

func (b *ChatBotImpl) handleUpdate(ctx context.Context, control chatController, update tgbotapi.Update) {
	event, ok := newEvent(update)
	if !ok {
		// We deliberately do not call errorHandler here: the event is empty,
		// so chatID is zero and there is nothing meaningful to send back.
		b.logger.Debugf("dropped unparseable update: %#v", update)
		return
	}

	b.logger.Debugf("got event: %#v", event)

	if control.LockChat(event.ChatID) {
		b.logger.Debugf("skip event (chat busy): %#v", event)
		return
	}
	defer control.UnlockChat(event.ChatID)

	layer := b.findAndWipeChatLayerHandler(event.ChatID)
	b.logger.Debugf("got layer: %#v", layer)
	event.lastLayer = layer

	handlerFunc := b.availableHandlerFromLayers(event, layer, b.defaultHandlerLayer)

	if err := b.applyMiddlewares(handlerFunc)(ctx, event); err != nil {
		b.errorHandler(ctx, event, err)
	}
}
