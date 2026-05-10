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

// --- A1: panic in handler is recovered, errorHandler still called ----------

func TestHandleUpdate_RecoversFromPanic(t *testing.T) {
	bot, _ := newTestBot()

	var captured atomic.Bool
	bot.RegisterErrorHandler(func(_ context.Context, _ Event, err error) {
		if err != nil && strings.Contains(err.Error(), "panic") {
			captured.Store(true)
		}
	})
	bot.RegisterCommand("/boom", func(_ context.Context, _ Event) error {
		panic("kaboom")
	})

	// Should not propagate the panic.
	c := newChatController(context.Background())
	bot.handleUpdate(context.Background(), c, tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text:     "/boom",
			Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 5}},
			Chat:     &tgbotapi.Chat{ID: 1},
			From:     &tgbotapi.User{ID: 1},
		},
	})

	if !captured.Load() {
		t.Fatal("error handler did not receive a panic-derived error")
	}
}

// --- A3: concurrent RegisterErrorHandler / RegisterDefaultHandler ----------

func TestRegister_ConcurrentRaceFree(_ *testing.T) {
	bot, _ := newTestBot()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				bot.RegisterErrorHandler(func(_ context.Context, _ Event, _ error) {})
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				bot.RegisterDefaultHandler(func(_ context.Context, _ Event) error { return nil })
			}
		}
	}()

	for i := 0; i < 500; i++ {
		_ = bot.getErrorHandler()
		_ = bot.availableHandlerFromLayers(Event{Kind: EventKindText, Text: "x"}, bot.defaultHandlerLayer, bot.defaultHandlerLayer)
	}
	close(stop)
	wg.Wait()
}

// --- A4: SendText applies parseMode ---------------------------------------

func TestSendText_AppliesParseMode(t *testing.T) {
	bot, mock := newTestBot()
	bot.parseMode = tgbotapi.ModeMarkdownV2

	if err := bot.SendText(1, "*bold*"); err != nil {
		t.Fatal(err)
	}
	msg, ok := mock.lastSent().(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("got %T", mock.lastSent())
	}
	if msg.ParseMode != tgbotapi.ModeMarkdownV2 {
		t.Fatalf("ParseMode=%q, want %q", msg.ParseMode, tgbotapi.ModeMarkdownV2)
	}
}

// --- A5: IsEmpty considers audioHandler -----------------------------------

func TestLayer_IsEmpty_VoiceCounts(t *testing.T) {
	l := newEmptyLayer()
	if !l.IsEmpty() {
		t.Fatal("fresh layer must be empty")
	}
	l.RegisterVoice(func(_ context.Context, _ Event) error { return nil })
	if l.IsEmpty() {
		t.Fatal("layer with voice handler must not be reported as empty")
	}
}

// --- A8: LoaderButton bails out after initial Send error ------------------

func TestLoaderButton_StopsLoopAfterInitialSendError(t *testing.T) {
	defer withShortTickers(t)()

	bot, mock := newTestBot()
	mock.sendErr = nil
	// Make the very first Send fail; LoaderButton must give up immediately
	// without entering the edit loop with MessageID=0.
	mock.sendErr = errSendFailed

	cancel := bot.LoaderButton(1, []string{"a", "b"})
	defer cancel()

	// Wait long enough for the (cancelled) loader to settle.
	time.Sleep(40 * time.Millisecond)

	// Only the single failed Send should have happened.
	if mock.sentCount() != 1 {
		t.Fatalf("expected exactly 1 send (the failed initial one), got %d", mock.sentCount())
	}
}

var errSendFailed = &sendError{msg: "send failed"}

type sendError struct{ msg string }

func (e *sendError) Error() string { return e.msg }

// --- A9: chatControllerSweep does not evict locks under TTL ---------------

func TestChatController_LongHandlerKeepsLock(t *testing.T) {
	defer withShortTickers(t)()

	// Make sweep frequent but TTL longer than the test window so a lock taken
	// before sweeps does not get evicted.
	chatControllerTTL.Store(int64(time.Second))
	chatControllerSweepInterval.Store(int64(5 * time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := newChatController(ctx)
	if !c.tryAcquire(7) {
		t.Fatal("first acquire must succeed")
	}

	// Several sweep ticks elapse, but the lock TTL is still ahead.
	time.Sleep(50 * time.Millisecond)

	if c.tryAcquire(7) {
		t.Fatal("lock was evicted while it should have stayed alive")
	}
	c.release(7)
}

// --- B5: RegisterText / RegisterButton no longer clobber each other -------

func TestRegisterTextAndButton_DoNotClobber(t *testing.T) {
	l := newEmptyLayer()

	called := ""
	l.RegisterText("Yes", func(_ context.Context, _ Event) error { called = "text"; return nil })
	l.RegisterButton("Yes", func(_ context.Context, _ Event) error { called = "button"; return nil })

	// Reply-keyboard buttons take priority.
	h := l.Handler(Event{Kind: EventKindText, Text: "Yes"})
	if h == nil {
		t.Fatal("nil handler")
	}
	_ = h(context.Background(), Event{})
	if called != "button" {
		t.Fatalf("button should override text on the same key; got %q", called)
	}

	// Both handlers are still independently registered.
	if _, ok := l.textHandler["Yes"]; !ok {
		t.Fatal("text handler was clobbered by button registration")
	}
	if _, ok := l.buttonTextHandler["Yes"]; !ok {
		t.Fatal("button handler missing")
	}
}

// --- B8: WithLayerTTL(0) does not silently change config -------------------

func TestWithLayerTTL_NonPositiveIgnored(t *testing.T) {
	bot, _ := newTestBot()
	bot.defaultTTL = 7 * time.Hour
	WithLayerTTL(0)(bot)
	if bot.defaultTTL != 7*time.Hour {
		t.Fatalf("defaultTTL=%v, want unchanged 7h", bot.defaultTTL)
	}
	WithLayerTTL(-1 * time.Second)(bot)
	if bot.defaultTTL != 7*time.Hour {
		t.Fatalf("defaultTTL=%v, want unchanged 7h", bot.defaultTTL)
	}
}

// --- B9: Event.String has no trailing newline ------------------------------

func TestEvent_StringNoTrailingNewline(t *testing.T) {
	ev := &Event{Text: "x"}
	got := ev.String()
	if strings.HasSuffix(got, "\n") {
		t.Fatalf("Event.String() must not end with newline; got %q", got)
	}
}

// --- C7: EventKind is exported and usable as a typed identifier ------------

func TestEventKind_Exported(t *testing.T) {
	k := EventKindText
	if k != "text" {
		t.Fatalf("EventKindText round-trip: %q", k)
	}
}

// --- WithUpdateConcurrency option ------------------------------------------

func TestWithUpdateConcurrency(t *testing.T) {
	bot, _ := newTestBot()
	WithUpdateConcurrency(8)(bot)
	if bot.updateConcurrency != 8 {
		t.Fatalf("got %d", bot.updateConcurrency)
	}
	WithUpdateConcurrency(0)(bot)
	if bot.updateConcurrency != 8 {
		t.Fatalf("zero must not overwrite, got %d", bot.updateConcurrency)
	}
	WithUpdateConcurrency(-5)(bot)
	if bot.updateConcurrency != 8 {
		t.Fatalf("negative must not overwrite, got %d", bot.updateConcurrency)
	}
}

// --- mainLoop: saturated semaphore drops update without blocking -----------

func TestMainLoop_SaturationDoesNotBlock(_ *testing.T) {
	bot, mock := newTestBot()
	bot.updateConcurrency = 1

	// One slow handler keeps the worker slot occupied.
	slow := make(chan struct{})
	bot.RegisterCommand("/slow", func(_ context.Context, _ Event) error {
		<-slow
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loopDone := make(chan struct{})
	go func() {
		_ = bot.mainLoop(ctx, mock.updates)
		close(loopDone)
	}()

	cmd := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text:     "/slow",
			Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 5}},
			Chat:     &tgbotapi.Chat{ID: 1},
			From:     &tgbotapi.User{ID: 1},
		},
	}
	mock.updates <- cmd
	// Give the handler a moment to occupy the slot.
	time.Sleep(20 * time.Millisecond)
	// Send another update. Concurrency=1 with the slot busy means the
	// dispatcher must drop it (rather than block).
	mock.updates <- cmd
	time.Sleep(20 * time.Millisecond)

	close(slow) // release first handler
	cancel()
	<-loopDone
}
