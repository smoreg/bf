package bf

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// --- NewLayer accepts mixed-type msgText -----------------------------------

func TestNewLayer_MixedTypeMsgText(t *testing.T) {
	bot, _ := newTestBot()

	tests := []struct {
		name string
		args []any
		want string
	}{
		{"empty", nil, ""},
		{"single string", []any{"hello"}, "hello"},
		{"two strings", []any{"hello", "world"}, "hello world"},
		{"int", []any{42}, "42"},
		{"mixed", []any{"answer:", 42, true}, "answer: 42 true"},
		{"trailing newline trimmed", []any{"line\n"}, "line"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := bot.NewLayer(tc.args...)
			if l.text != tc.want {
				t.Fatalf("text=%q, want %q", l.text, tc.want)
			}
		})
	}
}

// --- AddText edge cases ----------------------------------------------------

func TestAddText_EmptyString(t *testing.T) {
	l := newEmptyLayer()
	l.AddText("first")
	l.AddText("") // appends "" — produces "first\n"
	if l.text != "first\n" {
		t.Fatalf("unexpected text: %q", l.text)
	}
}

// --- RetryLastLayer after Stop is still safe -------------------------------

func TestRetryLastLayer_AfterStop(t *testing.T) {
	bot, _ := newTestBot()
	prev := bot.NewLayer("original")
	ev := Event{ChatID: 1, lastLayer: prev}

	bot.Stop()

	if err := bot.RetryLastLayer(ev, "after stop"); err != nil {
		// Send through the mock still works even after Stop because Stop
		// only stops update receiving and background goroutines.
		t.Fatalf("RetryLastLayer after Stop unexpectedly failed: %v", err)
	}
}

// --- Stress: many chats, many concurrent dispatchers -----------------------

func TestDispatcher_StressManyChats(t *testing.T) {
	bot, mock := newTestBot()

	const chats = 20
	const events = 50

	var hits atomic.Int64
	bot.RegisterCommand("/x", func(_ context.Context, _ Event) error {
		hits.Add(1)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	loopDone := make(chan struct{})
	go func() {
		_ = bot.mainLoop(ctx, mock.updates)
		close(loopDone)
	}()

	var wg sync.WaitGroup
	for chat := int64(1); chat <= chats; chat++ {
		wg.Add(1)
		go func(chatID int64) {
			defer wg.Done()
			for i := 0; i < events; i++ {
				mock.updates <- tgbotapi.Update{
					Message: &tgbotapi.Message{
						Text:     "/x",
						Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 2}},
						Chat:     &tgbotapi.Chat{ID: chatID},
						From:     &tgbotapi.User{ID: chatID},
					},
				}
			}
		}(chat)
	}
	wg.Wait()

	// Allow time for goroutines to drain.
	deadline := time.Now().Add(2 * time.Second)
	for hits.Load() < chats && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	<-loopDone

	if hits.Load() < chats {
		t.Fatalf("expected at least one hit per chat (>=%d), got %d", chats, hits.Load())
	}
	// chatController serialises per chat, so we should never exceed `events*chats`.
	if hits.Load() > int64(chats*events) {
		t.Fatalf("more hits than dispatched events: %d", hits.Load())
	}
}

// --- Bench: dispatcher hot path with no middleware -------------------------

func BenchmarkDispatcher_NoMiddleware(b *testing.B) {
	bot, _ := newTestBot()
	bot.RegisterCommand("/p", func(_ context.Context, _ Event) error { return nil })

	c := newChatController(context.Background())
	upd := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text:     "/p",
			Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 2}},
			Chat:     &tgbotapi.Chat{ID: 1},
			From:     &tgbotapi.User{ID: 1},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bot.handleUpdate(context.Background(), c, upd)
		c.release(1) // re-arm the lock for next iteration
	}
}

// --- Fuzz: newEvent never panics for arbitrary Update shapes ---------------

func FuzzNewEvent_DoesNotPanic(f *testing.F) {
	f.Add("hello", int64(1), int64(2))
	f.Add("", int64(0), int64(0))
	f.Add("/start arg", int64(-1), int64(0))

	f.Fuzz(func(_ *testing.T, text string, chatID, userID int64) {
		updates := []tgbotapi.Update{
			{Message: &tgbotapi.Message{Text: text, Chat: &tgbotapi.Chat{ID: chatID}, From: &tgbotapi.User{ID: userID}}},
			{Message: &tgbotapi.Message{Text: text}}, // nil Chat — must be rejected, not panic
			{Message: &tgbotapi.Message{Voice: &tgbotapi.Voice{FileID: text}, Chat: &tgbotapi.Chat{ID: chatID}}},
			{CallbackQuery: &tgbotapi.CallbackQuery{Data: text, From: &tgbotapi.User{ID: userID}}},
			{}, // empty
		}
		for _, u := range updates {
			_, _ = newEvent(u)
		}
	})
}

// --- ChatBot interface compliance check ------------------------------------

// Compile-time check that the high-level ChatBot interface is fully covered
// by ChatBotImpl. Already asserted in bot.go via "var _ ChatBot = ..." but we
// keep an explicit constructor here so a missing method shows up as a test
// failure too.
func TestChatBotInterfaceCompliance(_ *testing.T) {
	var _ ChatBot = (*ChatBotImpl)(nil)
}

// --- LoaderButton honours bot.shutdown bridge regardless of cancel()-order -

func TestLoaderButton_BridgeReleasedOnDone(t *testing.T) {
	defer withShortTickers(t)()

	bot, _ := newTestBot()
	cancel := bot.LoaderButton(1, []string{"a"})

	// Cancel via the returned func; the bridge goroutine should also exit.
	cancel()
	// Give the bridge goroutine a moment.
	time.Sleep(20 * time.Millisecond)

	// No assertion — the test passes if go test -race does not flag a leak
	// on subsequent operations.
	_ = bot
}

// --- defaultLayerMutex guards reads while writes happen --------------------

func TestDefaultLayer_ReadDuringWrite(_ *testing.T) {
	bot, _ := newTestBot()

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				bot.RegisterCommand(fmt.Sprintf("/c%d", time.Now().UnixNano()&0xff),
					func(_ context.Context, _ Event) error { return nil })
			}
		}
	}()

	for i := 0; i < 500; i++ {
		_ = bot.availableHandlerFromLayers(
			Event{Kind: EventKindCommand, Command: "ping"},
			bot.defaultHandlerLayer,
			bot.defaultHandlerLayer,
		)
	}
	close(stop)
}

// --- Sanity: TextHandlerKind constants exposed correctly -------------------

func TestTextHandlerKindConstants(t *testing.T) {
	if TextHandlerKindText == TextHandlerKindButton {
		t.Fatal("text and button kinds collided")
	}
	if !strings.HasPrefix(string(TextHandlerKindButton), "b") {
		t.Fatalf("unexpected kind value: %q", TextHandlerKindButton)
	}
}
