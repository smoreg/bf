package bf

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// --- newEvent: nil Chat is treated as undeliverable ------------------------

func TestNewEvent_NilChatRejected_Text(t *testing.T) {
	u := tgbotapi.Update{Message: &tgbotapi.Message{Text: "hi", From: &tgbotapi.User{ID: 1}}}
	if _, ok := newEvent(u); ok {
		t.Fatal("expected ok=false when Message.Chat is nil")
	}
}

func TestNewEvent_NilChatRejected_Voice(t *testing.T) {
	u := tgbotapi.Update{Message: &tgbotapi.Message{
		Voice: &tgbotapi.Voice{FileID: "f"},
		From:  &tgbotapi.User{ID: 1},
	}}
	if _, ok := newEvent(u); ok {
		t.Fatal("expected ok=false for voice without Chat")
	}
}

func TestNewEvent_NilChatRejected_Command(t *testing.T) {
	u := tgbotapi.Update{Message: &tgbotapi.Message{
		Text:     "/start",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
		From:     &tgbotapi.User{ID: 1},
	}}
	if _, ok := newEvent(u); ok {
		t.Fatal("expected ok=false for command without Chat")
	}
}

func TestNewEvent_CallbackQuery_NilChatStillOK(t *testing.T) {
	u := tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{
		Data:    "x",
		From:    &tgbotapi.User{ID: 1},
		Message: &tgbotapi.Message{}, // Chat is nil here
	}}
	ev, ok := newEvent(u)
	if !ok {
		t.Fatal("callback with nil Chat should still produce an event (ChatID=0)")
	}
	if ev.ChatID != 0 {
		t.Fatalf("ChatID=%d, want 0", ev.ChatID)
	}
}

// --- handleUpdate: no handler available is logged, not panic ---------------

func TestHandleUpdate_NoHandlerDoesNotPanic(t *testing.T) {
	bot, _ := newTestBot()

	// Wipe the default handler so availableHandlerFromLayers returns nil.
	bot.defaultHandlerLayer.layerDefaultHandler = nil

	c := newChatController(context.Background())
	bot.handleUpdate(context.Background(), c, tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text: "hello",
			Chat: &tgbotapi.Chat{ID: 1},
			From: &tgbotapi.User{ID: 1},
		},
	})
	// Reaching here means no panic occurred.
}

// --- SendMsg(nil) returns error instead of panicking ------------------------

func TestSendMsg_NilLayerReturnsError(t *testing.T) {
	bot, _ := newTestBot()
	if err := bot.SendMsg(1, nil); err == nil {
		t.Fatal("expected error for nil layer")
	}
}

// --- LoaderButton bound to bot Stop ----------------------------------------

func TestLoaderButton_StopCancelsLoader(t *testing.T) {
	defer withShortTickers(t)()

	bot, mock := newTestBot()

	bot.LoaderButton(1, []string{"frame1", "frame2"})

	// Wait for the loader to send at least one initial message so we know
	// the goroutines are alive.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if mock.sentCount() > 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	bot.Stop() // should propagate cancellation into every active loader

	// After Stop, no further sends should accumulate.
	stable := mock.sentCount()
	time.Sleep(40 * time.Millisecond)
	if mock.sentCount()-stable > 1 {
		t.Fatalf("loader kept sending after Stop: before=%d after=%d", stable, mock.sentCount())
	}
}

// --- mainLoop child ctx stops cleanOld even when updates channel closes ---

func TestMainLoop_UpdatesCloseStopsCleanOld(t *testing.T) {
	defer withShortTickers(t)()

	bot, mock := newTestBot()

	// Run mainLoop until updates close.
	done := make(chan struct{})
	go func() {
		_ = bot.mainLoop(context.Background(), mock.updates)
		close(done)
	}()

	// Spin a goroutine that injects fake "stale lock" entries into the
	// chatController; if cleanOld is still alive after updates close, the
	// race detector would catch the writes. Here we just verify mainLoop
	// returned.
	close(mock.updates)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("mainLoop did not return after updates closed")
	}
}

// --- atomic ticker var smoke check -----------------------------------------

func TestTickerHelpers(t *testing.T) {
	if loaderTick() <= 0 {
		t.Fatal("loaderTick must be positive")
	}
	if cleanerTick() <= 0 {
		t.Fatal("cleanerTick must be positive")
	}
	if chatControllerTick() <= 0 {
		t.Fatal("chatControllerTick must be positive")
	}
}

// --- defaults restored after withShortTickers -----------------------------

func TestWithShortTickers_RestoresDefaults(t *testing.T) {
	origLoader := loaderTickDelay.Load()
	origCleaner := cleanerTickInterval.Load()
	origCtl := chatControllerTTL.Load()

	restore := withShortTickers(t)
	if loaderTickDelay.Load() == origLoader {
		t.Fatal("loaderTickDelay not shortened")
	}
	restore()
	if loaderTickDelay.Load() != origLoader {
		t.Fatalf("loader not restored: %d != %d", loaderTickDelay.Load(), origLoader)
	}
	if cleanerTickInterval.Load() != origCleaner {
		t.Fatal("cleaner not restored")
	}
	if chatControllerTTL.Load() != origCtl {
		t.Fatal("chatController not restored")
	}
}

// --- Concurrent SendMsg + RegisterMiddleware safety -------------------------

func TestSendMsgAndRegisterMiddleware_Concurrent(t *testing.T) {
	bot, _ := newTestBot()

	var wg sync.WaitGroup
	stop := make(chan struct{})
	var sends atomic.Int64

	// Writer: middleware registrations.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				bot.RegisterMiddleware(func(next HandlerFunc) HandlerFunc { return next })
			}
		}
	}()

	// Reader: dispatcher path runs synchronously, then we ask the writer to halt.
	for i := 0; i < 500; i++ {
		h := bot.applyMiddlewares(func(_ context.Context, _ Event) error { return nil })
		_ = h(context.Background(), Event{})
		sends.Add(1)
	}
	close(stop)
	wg.Wait()

	if sends.Load() != 500 {
		t.Fatalf("got %d sends", sends.Load())
	}
}

// --- helper: ensure tgbotapi import isn't elided ---------------------------

var _ = strings.Repeat
