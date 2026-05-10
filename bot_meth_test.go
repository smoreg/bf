package bf

import (
	"context"
	"errors"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestNewLayer_TextEmptyAndNonEmpty(t *testing.T) {
	bot, _ := newTestBot()

	if got := bot.NewLayer(); got.text != "" {
		t.Fatalf("empty NewLayer text: want %q, got %q", "", got.text)
	}
	if got := bot.NewLayer("hello"); got.text != "hello" {
		t.Fatalf("NewLayer text: want %q, got %q", "hello", got.text)
	}
	if got := bot.NewLayer("a", "b"); got.text != "a b" {
		t.Fatalf("NewLayer text: want %q, got %q", "a b", got.text)
	}
}

func TestSendText(t *testing.T) {
	bot, mock := newTestBot()
	if err := bot.SendText(123, "hi"); err != nil {
		t.Fatal(err)
	}
	if mock.sentCount() != 1 {
		t.Fatalf("want 1 sent, got %d", mock.sentCount())
	}
	msg, ok := mock.lastSent().(tgbotapi.MessageConfig)
	if !ok || msg.Text != "hi" || msg.ChatID != 123 {
		t.Fatalf("unexpected msg: %+v", mock.lastSent())
	}
}

func TestSendText_PropagatesError(t *testing.T) {
	bot, mock := newTestBot()
	mock.sendErr = errors.New("boom")
	err := bot.SendText(1, "x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSendMsg_InlineButtons(t *testing.T) {
	bot, mock := newTestBot()
	l := bot.NewLayer("greet")
	l.RegisterIButton("OK", func(_ context.Context, _ Event) error { return nil })

	if err := bot.SendMsg(7, l); err != nil {
		t.Fatal(err)
	}

	msg, ok := mock.lastSent().(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("expected MessageConfig, got %T", mock.lastSent())
	}
	if msg.Text != "greet" {
		t.Fatalf("text: %q", msg.Text)
	}
	if _, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup); !ok {
		t.Fatalf("expected inline keyboard, got %T", msg.ReplyMarkup)
	}

	// Layer should now be installed for chat 7.
	bot.layersMutex.RLock()
	_, present := bot.chatHandlerLayers[7]
	bot.layersMutex.RUnlock()
	if !present {
		t.Fatal("layer not installed after SendMsg")
	}
}

func TestSendMsg_RegularButtons(t *testing.T) {
	bot, mock := newTestBot()
	l := bot.NewLayer("pick")
	l.RegisterButton("A", func(_ context.Context, _ Event) error { return nil })

	if err := bot.SendMsg(1, l); err != nil {
		t.Fatal(err)
	}

	msg, ok := mock.lastSent().(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("got %T", mock.lastSent())
	}
	if _, ok := msg.ReplyMarkup.(tgbotapi.ReplyKeyboardMarkup); !ok {
		t.Fatalf("expected reply keyboard, got %T", msg.ReplyMarkup)
	}
}

func TestSendMsg_MixedButtonsErrors(t *testing.T) {
	bot, _ := newTestBot()
	l := bot.NewLayer("x")
	l.RegisterIButton("inline", func(_ context.Context, _ Event) error { return nil })
	l.RegisterButton("regular", func(_ context.Context, _ Event) error { return nil })

	if err := bot.SendMsg(1, l); err == nil {
		t.Fatal("expected error mixing inline and regular buttons")
	}
}

func TestSendMsg_PropagatesSendError(t *testing.T) {
	bot, mock := newTestBot()
	mock.sendErr = errors.New("net")
	if err := bot.SendMsg(1, bot.NewLayer("x")); err == nil {
		t.Fatal("expected error")
	}
}

func TestRetryLastLayer_NoPrevious(t *testing.T) {
	bot, _ := newTestBot()
	if err := bot.RetryLastLayer(Event{ChatID: 1}, ""); err == nil {
		t.Fatal("expected error when no previous layer")
	}
}

func TestRetryLastLayer_DoesNotMutateOriginal(t *testing.T) {
	bot, _ := newTestBot()
	prev := bot.NewLayer("orig")
	ev := Event{ChatID: 5}
	ev.lastLayer = prev

	if err := bot.RetryLastLayer(ev, "new"); err != nil {
		t.Fatal(err)
	}
	if prev.text != "orig" {
		t.Fatalf("original layer text mutated: %q", prev.text)
	}
}

func TestRetryLastLayer_KeepsTextWhenEmpty(t *testing.T) {
	bot, mock := newTestBot()
	prev := bot.NewLayer("keep")
	ev := Event{ChatID: 5, lastLayer: prev}

	if err := bot.RetryLastLayer(ev, ""); err != nil {
		t.Fatal(err)
	}
	msg, ok := mock.lastSent().(tgbotapi.MessageConfig)
	if !ok || msg.Text != "keep" {
		t.Fatalf("expected original text 'keep', got %+v", mock.lastSent())
	}
}

func TestRegisterCommand_OnDefaultLayer(t *testing.T) {
	bot, _ := newTestBot()
	hit := false
	bot.RegisterCommand("/ping", func(_ context.Context, _ Event) error { hit = true; return nil })

	h := bot.defaultHandlerLayer.Handler(Event{Kind: EventKindCommand, Command: "ping"})
	if h == nil {
		t.Fatal("nil handler")
	}
	_ = h(context.Background(), Event{})
	if !hit {
		t.Fatal("handler not invoked")
	}
}

func TestRegisterButton_AndAudio(t *testing.T) {
	bot, _ := newTestBot()
	bot.RegisterButton("X", func(_ context.Context, _ Event) error { return nil })
	bot.RegisterAudio(func(_ context.Context, _ Event) error { return nil })

	if h := bot.defaultHandlerLayer.Handler(Event{Kind: EventKindText, Text: "X"}); h == nil {
		t.Fatal("button handler not registered")
	}
	if h := bot.defaultHandlerLayer.Handler(Event{Kind: EventKindVoice}); h == nil {
		t.Fatal("audio handler not registered")
	}
}

func TestRegisterErrorHandlerAndMiddleware(t *testing.T) {
	bot, _ := newTestBot()
	called := false
	bot.RegisterErrorHandler(func(_ context.Context, _ Event, _ error) { called = true })
	bot.errorHandler(context.Background(), Event{}, errors.New("x"))
	if !called {
		t.Fatal("custom error handler not stored")
	}

	bot.RegisterMiddleware(func(next HandlerFunc) HandlerFunc { return next })
	if len(bot.middlewares) != 1 {
		t.Fatalf("middleware not appended, got %d", len(bot.middlewares))
	}
}

func TestSelfUserName(t *testing.T) {
	bot, _ := newTestBot()
	if bot.SelfUserName() != "mock_bot" {
		t.Fatalf("got %q", bot.SelfUserName())
	}
}

func TestGetFileURL(t *testing.T) {
	bot, mock := newTestBot()
	mock.fileURLs["abc"] = "https://t.me/file/abc"

	url, err := bot.GetFileURL("abc")
	if err != nil || url != "https://t.me/file/abc" {
		t.Fatalf("got %q, %v", url, err)
	}

	mock.fileURLErr = errors.New("nope")
	if _, err := bot.GetFileURL("abc"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoaderButton_EmptyScreen(t *testing.T) {
	bot, _ := newTestBot()
	cancel := bot.LoaderButton(1, nil)
	cancel()
}

func TestLoaderButton_CancelStopsGoroutine(t *testing.T) {
	bot, mock := newTestBot()
	cancel := bot.LoaderButton(1, []string{"loading"})
	// Allow the goroutine to send the initial message.
	time.Sleep(50 * time.Millisecond)
	cancel()
	// Wait briefly to ensure no further sends after cancel.
	before := mock.sentCount()
	time.Sleep(100 * time.Millisecond)
	after := mock.sentCount()
	if after-before > 1 {
		t.Fatalf("loader continued sending after cancel: before=%d after=%d", before, after)
	}
}
