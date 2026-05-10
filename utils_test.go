package bf

import (
	"context"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestValidateConfiguration(t *testing.T) {
	bot, _ := newTestBot()
	if err := bot.validateConfiguration(); err != nil {
		t.Fatalf("default newTestBot must validate, got: %v", err)
	}

	bot.errorHandler = nil
	if err := bot.validateConfiguration(); err == nil {
		t.Fatal("expected error when errorHandler nil")
	}

	bot.errorHandler = bot.defaultErrorHandler
	bot.defaultHandlerLayer = nil
	if err := bot.validateConfiguration(); err == nil {
		t.Fatal("expected error when defaultHandlerLayer nil")
	}

	bot, _ = newTestBot()
	bot.defaultHandlerLayer.layerDefaultHandler = nil
	if err := bot.validateConfiguration(); err == nil {
		t.Fatal("expected error when default handler nil")
	}
}

func TestBuildInlineKeyboard_Default(t *testing.T) {
	bot, _ := newTestBot()
	btns := []tgbotapi.InlineKeyboardButton{
		{Text: "a"}, {Text: "b"}, {Text: "c"},
	}
	got := bot.buildInlineKeyboard(btns, false)
	if len(got.InlineKeyboard) != 3 {
		t.Fatalf("want 3 rows, got %d", len(got.InlineKeyboard))
	}
}

func TestBuildInlineKeyboard_RowMode(t *testing.T) {
	bot, _ := newTestBot()
	btns := []tgbotapi.InlineKeyboardButton{{Text: "a"}, {Text: "b"}, {Text: "c"}}
	got := bot.buildInlineKeyboard(btns, true)
	if len(got.InlineKeyboard) != 2 {
		t.Fatalf("rowMode: want 2 rows, got %d", len(got.InlineKeyboard))
	}
	if len(got.InlineKeyboard[0]) != 2 || len(got.InlineKeyboard[1]) != 1 {
		t.Fatalf("rowMode shape unexpected: %+v", got.InlineKeyboard)
	}
}

func TestBuildInlineKeyboard_RowMode_OneButton_NoSplit(t *testing.T) {
	bot, _ := newTestBot()
	btns := []tgbotapi.InlineKeyboardButton{{Text: "a"}}
	got := bot.buildInlineKeyboard(btns, true)
	if len(got.InlineKeyboard) != 1 {
		t.Fatalf("rowMode with 1 btn: want 1 row, got %d", len(got.InlineKeyboard))
	}
}

func TestApplyMiddlewares_Order(t *testing.T) {
	bot, _ := newTestBot()
	var trace []string
	mw := func(name string) MiddlewareFunc {
		return func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, ev Event) error {
				trace = append(trace, "in:"+name)
				err := next(ctx, ev)
				trace = append(trace, "out:"+name)
				return err
			}
		}
	}
	bot.middlewares = []MiddlewareFunc{mw("a"), mw("b")}

	final := func(_ context.Context, _ Event) error {
		trace = append(trace, "handler")
		return nil
	}

	wrapped := bot.applyMiddlewares(final)
	_ = wrapped(context.Background(), Event{})

	want := []string{"in:b", "in:a", "handler", "out:a", "out:b"}
	if len(trace) != len(want) {
		t.Fatalf("len mismatch: got %v want %v", trace, want)
	}
	for i := range want {
		if trace[i] != want[i] {
			t.Fatalf("trace[%d]: got %q want %q", i, trace[i], want[i])
		}
	}
}

func TestAvailableHandlerFromLayers_FallsBackToDefault(t *testing.T) {
	bot, _ := newTestBot()
	chat := bot.NewLayer()

	hit := ""
	bot.defaultHandlerLayer.RegisterText("hi", func(_ context.Context, _ Event) error { hit = "default"; return nil })

	h := bot.availableHandlerFromLayers(Event{Kind: EventKindText, Text: "hi"}, chat, bot.defaultHandlerLayer)
	if h == nil {
		t.Fatal("nil handler")
	}
	_ = h(context.Background(), Event{})
	if hit != "default" {
		t.Fatalf("want default, got %q", hit)
	}
}

func TestGetAndDeleteLayer_Concurrent(t *testing.T) {
	bot, _ := newTestBot()
	for i := int64(0); i < 100; i++ {
		bot.setLayer(bot.NewLayer(), i)
	}

	var wg sync.WaitGroup
	wg.Add(100)
	for i := int64(0); i < 100; i++ {
		go func(id int64) {
			defer wg.Done()
			_, _ = bot.getAndDeleteLayer(id)
		}(i)
	}
	wg.Wait()

	if len(bot.chatHandlerLayers) != 0 {
		t.Fatalf("expected all layers removed, got %d", len(bot.chatHandlerLayers))
	}
}

func TestSetLayerAndRetrieve(t *testing.T) {
	bot, _ := newTestBot()
	l := bot.NewLayer()
	bot.setLayer(l, 99)
	got, ok := bot.getAndDeleteLayer(99)
	if !ok || got != l {
		t.Fatal("layer not retrievable after setLayer")
	}
	if _, ok := bot.getAndDeleteLayer(99); ok {
		t.Fatal("layer not deleted after retrieval")
	}
}

func TestCleaner_RemovesExpired(t *testing.T) {
	bot, _ := newTestBot()
	expired := bot.NewLayer()
	expired.ttl = time.Now().Add(-time.Hour)
	bot.setLayer(expired, 1)

	fresh := bot.NewLayer()
	fresh.ttl = time.Now().Add(time.Hour)
	bot.setLayer(fresh, 2)

	// Run one cleaner iteration manually (avoid 10-min ticker).
	bot.layersMutex.Lock()
	for chatID, layer := range bot.chatHandlerLayers {
		if layer.IsExpired() {
			delete(bot.chatHandlerLayers, chatID)
		}
	}
	bot.layersMutex.Unlock()

	if _, ok := bot.chatHandlerLayers[1]; ok {
		t.Fatal("expired layer not removed")
	}
	if _, ok := bot.chatHandlerLayers[2]; !ok {
		t.Fatal("fresh layer removed by mistake")
	}
}
