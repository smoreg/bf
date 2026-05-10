package bf

import (
	"context"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func newEmptyLayer() *HandlerLayer {
	return &HandlerLayer{
		commandHandler:    map[string]CommandHandler{},
		textHandler:       map[string]TextHandler{},
		buttonTextHandler: map[string]TextHandler{},
		buttonHandler:     map[string]InlineButtonHandler{},
		ttl:               time.Now().Add(time.Hour),
	}
}

func TestLayer_RegisterAndDispatchText(t *testing.T) {
	l := newEmptyLayer()

	called := ""
	l.RegisterText("hi", func(_ context.Context, _ Event) error { called = "exact"; return nil })
	l.RegisterText(AnyText, func(_ context.Context, _ Event) error { called = "any"; return nil })

	if h := l.Handler(Event{Kind: EventKindText, Text: "hi"}); h == nil {
		t.Fatal("expected exact handler, got nil")
	} else {
		_ = h(context.Background(), Event{})
		if called != "exact" {
			t.Fatalf("want exact, got %q", called)
		}
	}

	called = ""
	if h := l.Handler(Event{Kind: EventKindText, Text: "anything-else"}); h == nil {
		t.Fatal("expected AnyText fallback handler")
	} else {
		_ = h(context.Background(), Event{})
		if called != "any" {
			t.Fatalf("want any, got %q", called)
		}
	}
}

func TestLayer_DispatchCommand(t *testing.T) {
	l := newEmptyLayer()
	hit := false
	l.RegisterCommand("/start", func(_ context.Context, _ Event) error { hit = true; return nil })

	h := l.Handler(Event{Kind: EventKindCommand, Command: "start"})
	if h == nil {
		t.Fatal("nil handler for command")
	}
	_ = h(context.Background(), Event{})
	if !hit {
		t.Fatal("command handler not invoked")
	}
}

func TestLayer_DispatchInlineButton(t *testing.T) {
	l := newEmptyLayer()
	l.RegisterIButton("OK", func(_ context.Context, _ Event) error { return nil })

	var btnID string
	for id := range l.buttonHandler {
		btnID = id
	}
	if btnID == "" {
		t.Fatal("button not registered")
	}

	if h := l.Handler(Event{Kind: EventKindInlineButton, Button: btnID}); h == nil {
		t.Fatal("button handler nil")
	}
	if h := l.Handler(Event{Kind: EventKindInlineButton, Button: "missing"}); h != nil {
		t.Fatal("expected nil for unknown button")
	}
}

func TestLayer_DispatchVoice(t *testing.T) {
	l := newEmptyLayer()

	if h := l.Handler(Event{Kind: EventKindVoice}); h != nil {
		t.Fatal("expected nil voice handler before register")
	}

	l.RegisterVoice(func(_ context.Context, _ Event) error { return nil })
	if h := l.Handler(Event{Kind: EventKindVoice}); h == nil {
		t.Fatal("voice handler nil after register")
	}
}

func TestLayer_FallbackToDefault(t *testing.T) {
	l := newEmptyLayer()
	l.layerDefaultHandler = func(_ context.Context, _ Event) error { return nil }

	if h := l.Handler(Event{Kind: EventKindCommand, Command: "unknown"}); h == nil {
		t.Fatal("expected default handler fallback")
	}
}

func TestLayer_IsExpired(t *testing.T) {
	l := newEmptyLayer()
	l.ttl = time.Now().Add(-time.Second)
	if !l.IsExpired() {
		t.Fatal("expected expired")
	}
	l.ttl = time.Now().Add(time.Hour)
	if l.IsExpired() {
		t.Fatal("expected not expired")
	}
}

func TestLayer_IsEmpty(t *testing.T) {
	l := newEmptyLayer()
	if !l.IsEmpty() {
		t.Fatal("fresh layer should be empty")
	}
	l.RegisterText("x", func(_ context.Context, _ Event) error { return nil })
	if l.IsEmpty() {
		t.Fatal("layer with handler should not be empty")
	}
}

func TestLayer_AddText(t *testing.T) {
	l := newEmptyLayer()
	l.AddText("first")
	if l.text != "first" {
		t.Fatalf("want %q, got %q", "first", l.text)
	}
	l.AddText("second")
	if l.text != "first\nsecond" {
		t.Fatalf("want %q, got %q", "first\nsecond", l.text)
	}
}

func TestLayer_RegisterButton_OrderWeight(t *testing.T) {
	l := newEmptyLayer()
	l.RegisterButton("a", func(_ context.Context, _ Event) error { return nil })
	l.RegisterButton("b", func(_ context.Context, _ Event) error { return nil })
	l.RegisterButton("c", func(_ context.Context, _ Event) error { return nil })

	got := l.sortedButtonsSlice()
	if len(got) != 3 {
		t.Fatalf("want 3 sorted buttons, got %d", len(got))
	}
	if got[0].text != "a" || got[1].text != "b" || got[2].text != "c" {
		t.Fatalf("unexpected order: %+v", got)
	}
}

func TestLayer_SortedIButtons(t *testing.T) {
	l := newEmptyLayer()
	l.RegisterIButton("first", func(_ context.Context, _ Event) error { return nil })
	l.RegisterIButton("second", func(_ context.Context, _ Event) error { return nil })

	got := l.sortedIButtonsSlice()
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].orderWeight >= got[1].orderWeight {
		t.Fatal("orderWeight not ascending")
	}
}

func TestLayer_RegisterIButtonURL_NoHandler(t *testing.T) {
	l := newEmptyLayer()
	l.RegisterIButtonURL("Open", "https://example.com")
	if len(l.buttonHandler) != 1 {
		t.Fatalf("want 1 button, got %d", len(l.buttonHandler))
	}
	for _, h := range l.buttonHandler {
		if h.handlerFunc != nil {
			t.Fatal("URL button must have no handler")
		}
		if h.button.URL == nil || *h.button.URL != "https://example.com" {
			t.Fatal("URL not set on button")
		}
	}
}

func TestLayer_RegisterIButtonSwitch(t *testing.T) {
	l := newEmptyLayer()
	l.RegisterIButtonSwitch("share", "query", func(_ context.Context, _ Event) error { return nil })
	if len(l.buttonHandler) != 1 {
		t.Fatalf("want 1 button, got %d", len(l.buttonHandler))
	}
}

func TestLayer_SetIButtonRowMode(t *testing.T) {
	l := newEmptyLayer()
	if l.rowMode {
		t.Fatal("rowMode should default false")
	}
	l.SetIButtonRowMode()
	if !l.rowMode {
		t.Fatal("rowMode not set")
	}
}

func TestLayer_NoMatchReturnsNil(t *testing.T) {
	l := newEmptyLayer()
	cases := []Event{
		{Kind: EventKindText, Text: "x"},
		{Kind: EventKindCommand, Command: "x"},
		{Kind: EventKindInlineButton, Button: "x"},
		{Kind: EventKindVoice},
		{Kind: "unknown"},
	}
	for _, ev := range cases {
		if h := l.Handler(ev); h != nil {
			t.Fatalf("want nil handler for %v", ev)
		}
	}
}

// Ensure tgbotapi types still link when handler set.
var _ = tgbotapi.InlineKeyboardButton{}
