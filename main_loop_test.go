package bf

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestChatController_LockUnlock(t *testing.T) {
	c := newChatController(context.Background())

	if c.LockChat(1) {
		t.Fatal("first lock must succeed")
	}
	if !c.LockChat(1) {
		t.Fatal("second lock on same chat must report busy")
	}
	c.UnlockChat(1)
	if c.LockChat(1) {
		t.Fatal("after unlock, lock must succeed again")
	}
}

func TestChatController_Concurrent(t *testing.T) {
	c := newChatController(context.Background())

	var locks int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !c.LockChat(42) {
				atomic.AddInt64(&locks, 1)
				time.Sleep(10 * time.Millisecond)
				c.UnlockChat(42)
			}
		}()
	}
	wg.Wait()

	if locks == 0 {
		t.Fatal("at least one goroutine should have acquired the lock")
	}
}

func TestMainLoop_StopsOnContextCancel(t *testing.T) {
	bot, mock := newTestBot()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = bot.mainLoop(ctx, mock.updates)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("mainLoop did not exit on context cancel")
	}
}

func TestMainLoop_StopsOnUpdatesClose(t *testing.T) {
	bot, mock := newTestBot()
	done := make(chan struct{})
	go func() {
		_ = bot.mainLoop(context.Background(), mock.updates)
		close(done)
	}()

	close(mock.updates)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("mainLoop did not exit when updates channel closed")
	}
}

func TestMainLoop_DispatchesEvents(t *testing.T) {
	bot, mock := newTestBot()

	var hit atomic.Int32
	bot.RegisterCommand("/ping", func(_ context.Context, _ Event) error {
		hit.Add(1)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = bot.mainLoop(ctx, mock.updates) }()

	mock.updates <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text:     "/ping",
			Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 5}},
			Chat:     &tgbotapi.Chat{ID: 1},
			From:     &tgbotapi.User{ID: 1},
		},
	}

	deadline := time.Now().Add(time.Second)
	for hit.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if hit.Load() != 1 {
		t.Fatalf("handler not called, hit=%d", hit.Load())
	}
}

func TestMainLoop_DropsUnparseableUpdate(t *testing.T) {
	bot, mock := newTestBot()

	errCalls := atomic.Int32{}
	bot.RegisterErrorHandler(func(_ context.Context, _ Event, _ error) { errCalls.Add(1) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = bot.mainLoop(ctx, mock.updates) }()

	mock.updates <- tgbotapi.Update{}
	time.Sleep(50 * time.Millisecond)
	if errCalls.Load() != 0 {
		t.Fatalf("errorHandler called for unparseable update; got %d", errCalls.Load())
	}
}

func TestHandleUpdate_BusyChatSkipped(t *testing.T) {
	bot, _ := newTestBot()
	c := newChatController(context.Background())
	c.LockChat(1)

	var hit atomic.Int32
	bot.RegisterCommand("/x", func(_ context.Context, _ Event) error { hit.Add(1); return nil })

	upd := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text:     "/x",
			Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 2}},
			Chat:     &tgbotapi.Chat{ID: 1},
			From:     &tgbotapi.User{ID: 1},
		},
	}
	bot.handleUpdate(context.Background(), c, upd)

	if hit.Load() != 0 {
		t.Fatal("handler should be skipped on busy chat")
	}
}
