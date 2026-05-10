package bf

import (
	"context"
	"errors"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type capturingLogger struct{ errs []string }

func (c *capturingLogger) Debug(...any)                      {}
func (c *capturingLogger) Debugf(string, ...any)             {}
func (c *capturingLogger) Info(...any)                       {}
func (c *capturingLogger) Infof(string, ...any)              {}
func (c *capturingLogger) Warn(...any)                       {}
func (c *capturingLogger) Warnf(string, ...any)              {}
func (c *capturingLogger) Error(args ...any)                 { c.errs = append(c.errs, "err") }
func (c *capturingLogger) Errorf(format string, args ...any) { c.errs = append(c.errs, format) }

func TestDefaultErrorHandler_LogsErr(t *testing.T) {
	bot, _ := newTestBot()
	cap := &capturingLogger{}
	bot.logger = cap
	bot.defaultErrorHandler(context.Background(), Event{ChatID: 1}, errors.New("boom"))
	if len(cap.errs) == 0 {
		t.Fatal("error not logged")
	}
}

func TestDefaultEventHandler_DebugOff_NoSend(t *testing.T) {
	bot, mock := newTestBot()
	if err := bot.defaultEventHandler(context.Background(), Event{ChatID: 1}); err != nil {
		t.Fatal(err)
	}
	if mock.sentCount() != 0 {
		t.Fatal("debug=false must not send")
	}
}

func TestDefaultEventHandler_DebugOn_SendsJSON(t *testing.T) {
	bot, mock := newTestBot()
	bot.debug = true
	if err := bot.defaultEventHandler(context.Background(), Event{ChatID: 9, Text: "x"}); err != nil {
		t.Fatal(err)
	}
	if mock.sentCount() != 1 {
		t.Fatalf("want 1 sent, got %d", mock.sentCount())
	}
	msg, ok := mock.lastSent().(tgbotapi.MessageConfig)
	if !ok || msg.ChatID != 9 {
		t.Fatalf("bad message: %+v", mock.lastSent())
	}
}
