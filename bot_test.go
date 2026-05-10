package bf

import (
	"context"
	"testing"
)

func TestStop_Idempotent(t *testing.T) {
	bot, mock := newTestBot()
	bot.Stop()
	bot.Stop() // must not panic via double close
	if !mock.stopped.Load() {
		t.Fatal("StopReceivingUpdates not invoked")
	}
}

func TestStop_StopsCleaner(t *testing.T) {
	bot, _ := newTestBot()
	done := make(chan struct{})
	go func() {
		bot.cleaner()
		close(done)
	}()

	bot.Stop()

	select {
	case <-done:
	case <-mustTimeout(t):
		t.Fatal("cleaner did not exit on Stop")
	}
}

// NewBot itself requires a real Telegram API key (network call),
// so we cover the post-construction wiring via newTestBot in other tests.
// This test only exercises the early-failure branch.
func TestNewBot_InvalidKey(t *testing.T) {
	if _, err := NewBot(""); err == nil {
		t.Fatal("expected error on empty API key")
	}
}

func TestStart_ValidatesConfiguration(t *testing.T) {
	bot, _ := newTestBot()
	bot.errorHandler = nil
	if err := bot.Start(context.Background()); err == nil {
		t.Fatal("expected validation error")
	}
}
