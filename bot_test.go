package bf

import (
	"context"
	"testing"
	"time"
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
	case <-time.After(2 * time.Second):
		t.Fatal("cleaner did not exit on Stop")
	}
}

// NewBot itself requires a real Telegram API key. We cover the early-failure
// branch with an empty key (rejected without a network call) and the success
// branch via NewBotWithEndpoint against a stub server in coverage_extra_test.go.
func TestNewBot_InvalidKey(t *testing.T) {
	bot, err := NewBot("")
	if err == nil {
		t.Fatal("expected error on empty API key")
	}
	if bot != nil {
		t.Fatal("expected nil bot on error")
	}
}

func TestStart_ValidatesConfiguration(t *testing.T) {
	bot, _ := newTestBot()
	bot.errorHandler = nil
	if err := bot.Start(context.Background()); err == nil {
		t.Fatal("expected validation error")
	}
}
