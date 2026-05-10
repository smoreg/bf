package bf

import (
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestWithDebug(t *testing.T) {
	bot, _ := newTestBot()
	WithDebug()(bot)
	if !bot.debug {
		t.Fatal("WithDebug did not set debug=true")
	}
}

func TestWithLogger_NilIgnored(t *testing.T) {
	bot, _ := newTestBot()
	prev := bot.logger
	WithLogger(nil)(bot)
	if bot.logger != prev {
		t.Fatal("WithLogger(nil) should not overwrite logger")
	}
}

func TestWithLogger_Sets(t *testing.T) {
	bot, _ := newTestBot()
	custom := noopLogger{}
	WithLogger(custom)(bot)
	if bot.logger != custom {
		t.Fatal("WithLogger did not store provided logger")
	}
}

func TestWithParseMode(t *testing.T) {
	bot, _ := newTestBot()
	WithParseMode(tgbotapi.ModeMarkdownV2)(bot)
	if bot.parseMode != tgbotapi.ModeMarkdownV2 {
		t.Fatalf("got %q", bot.parseMode)
	}
}

func TestWithLayerTTL(t *testing.T) {
	bot, _ := newTestBot()
	WithLayerTTL(5 * time.Minute)(bot)
	if bot.defaultTTL != 5*time.Minute {
		t.Fatalf("got %v", bot.defaultTTL)
	}

	// Negative / zero are ignored.
	WithLayerTTL(0)(bot)
	if bot.defaultTTL != 5*time.Minute {
		t.Fatal("zero TTL must be ignored")
	}
}
