package bf

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// chatControllerTTL is how long a per-chat lock stays alive before being
// reclaimed by the background sweeper. Long enough that genuinely long
// handlers do not get their lock yanked out from under them.
var chatControllerTTL atomic.Int64

// chatControllerSweepInterval is how often the sweeper wakes up to check for
// stale locks. Independent from the TTL so a long-running handler is not at
// risk of having its lock evicted just because the sweeper ticks frequently.
var chatControllerSweepInterval atomic.Int64

func init() {
	chatControllerTTL.Store(int64(10 * time.Minute))
	chatControllerSweepInterval.Store(int64(1 * time.Minute))
}

func chatControllerTick() time.Duration {
	return time.Duration(chatControllerSweepInterval.Load())
}

func chatControllerLockTTL() time.Duration {
	return time.Duration(chatControllerTTL.Load())
}

// defaultUpdateConcurrency caps how many updates are processed in parallel.
// Each ChatBotImpl owns a semaphore sized to this value; a single noisy chat
// channel cannot spawn an unbounded number of goroutines.
const defaultUpdateConcurrency = 256

// chatController serialises message processing per chat: while one update from
// a given chat is being handled, subsequent updates from the same chat are
// dropped. This prevents handler interleaving for the same user.
type chatController struct {
	userInWork map[int64]time.Time
	mux        *sync.Mutex
}

// cleanOld evicts stale chat locks. Stops when ctx is cancelled.
func (c chatController) cleanOld(ctx context.Context) {
	ticker := time.NewTicker(chatControllerTick())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ttl := chatControllerLockTTL()
			c.mux.Lock()
			for userID, lastTime := range c.userInWork {
				if lastTime.Add(ttl).Before(time.Now()) {
					delete(c.userInWork, userID)
				}
			}
			c.mux.Unlock()
		}
	}
}

// tryAcquire returns true if the caller acquired the per-chat lock and should
// proceed; false means another goroutine is already processing that chat and
// the caller should drop the update. Callers that get true must call release.
func (c chatController) tryAcquire(chatID int64) bool {
	c.mux.Lock()
	defer c.mux.Unlock()

	if _, ok := c.userInWork[chatID]; ok {
		return false
	}
	c.userInWork[chatID] = time.Now()
	return true
}

func (c chatController) release(chatID int64) {
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
	// Derive a child context so chatController.cleanOld is guaranteed to
	// terminate even if mainLoop exits because the updates channel closed
	// (rather than because ctx was cancelled).
	loopCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	control := newChatController(loopCtx)

	// Bounded worker concurrency. We do not block in the producer loop —
	// instead we drop the update with a log when the semaphore is full,
	// keeping mainLoop responsive to ctx cancellation.
	sem := make(chan struct{}, b.updateConcurrency)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			select {
			case sem <- struct{}{}:
				go func(u tgbotapi.Update) {
					defer func() { <-sem }()
					b.handleUpdate(loopCtx, control, u)
				}(update)
			default:
				b.logger.Warnf("dispatcher saturated; dropping update")
			}
		}
	}
}

func (b *ChatBotImpl) handleUpdate(ctx context.Context, control chatController, update tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("handler panic: %v", r)
			b.logger.Errorf("recovered from panic in handler: %v\n%s", r, debug.Stack())
			// Best-effort report to the registered error handler. The event we
			// recovered with may itself be partially populated.
			if eh := b.getErrorHandler(); eh != nil {
				ev, _ := newEvent(update)
				eh(ctx, ev, err)
			}
		}
	}()

	event, ok := newEvent(update)
	if !ok {
		// We deliberately do not call errorHandler here: the event is empty,
		// so chatID is zero and there is nothing meaningful to send back.
		b.logger.Debugf("dropped unparseable update: %#v", update)
		return
	}

	b.logger.Debugf("got event: %#v", event)

	if !control.tryAcquire(event.ChatID) {
		b.logger.Debugf("skip event (chat busy): %#v", event)
		return
	}
	defer control.release(event.ChatID)

	layer := b.findAndWipeChatLayerHandler(event.ChatID)
	b.logger.Debugf("got layer: %#v", layer)
	event.lastLayer = layer

	handlerFunc := b.availableHandlerFromLayers(event, layer, b.defaultHandlerLayer)
	if handlerFunc == nil {
		// Both the chat layer and the default layer returned nil. This happens
		// when a URL-only inline button is somehow tapped, or when the user
		// cleared the default handler at runtime. Drop the event with a log
		// rather than panicking on a nil call.
		b.logger.Debugf("no handler for event: %#v", event)
		return
	}

	if err := b.applyMiddlewares(handlerFunc)(ctx, event); err != nil {
		if eh := b.getErrorHandler(); eh != nil {
			eh(ctx, event, err)
		}
	}
}
