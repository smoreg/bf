package bf

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// withShortTickers temporarily shortens background tickers so loop branches
// can be exercised in milliseconds. Returns a restore func to defer.
// Reads/writes go through atomic.Int64 so concurrent goroutines reading the
// values do not race the test that swaps them back.
func withShortTickers(t *testing.T) func() {
	t.Helper()
	origCleaner := cleanerTickInterval.Load()
	origController := chatControllerTTL.Load()
	origLoader := loaderTickDelay.Load()

	short := int64(5 * time.Millisecond)
	cleanerTickInterval.Store(short)
	chatControllerTTL.Store(short)
	loaderTickDelay.Store(short)

	return func() {
		cleanerTickInterval.Store(origCleaner)
		chatControllerTTL.Store(origController)
		loaderTickDelay.Store(origLoader)
	}
}

// --- cleaner ----------------------------------------------------------------

func TestCleaner_RemovesExpiredOnTick(t *testing.T) {
	defer withShortTickers(t)()

	bot, _ := newTestBot()

	expired := bot.NewLayer()
	expired.ttl = time.Now().Add(-time.Hour)
	bot.setLayer(expired, 1)

	go bot.cleaner()
	defer bot.Stop()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		bot.layersMutex.RLock()
		_, present := bot.chatHandlerLayers[1]
		bot.layersMutex.RUnlock()
		if !present {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("expired layer not swept by cleaner")
}

func TestCleaner_StopsOnShutdown(t *testing.T) {
	defer withShortTickers(t)()

	bot, _ := newTestBot()
	done := make(chan struct{})
	go func() {
		bot.cleaner()
		close(done)
	}()

	bot.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleaner did not exit on Stop")
	}
}

// --- chatController.cleanOld -----------------------------------------------

func TestChatController_CleanOldEvictsStaleLocks(t *testing.T) {
	defer withShortTickers(t)()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := newChatController(ctx)

	// Insert a stale lock manually.
	c.mux.Lock()
	c.userInWork[42] = time.Now().Add(-time.Hour)
	c.mux.Unlock()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		c.mux.Lock()
		_, present := c.userInWork[42]
		c.mux.Unlock()
		if !present {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("stale lock not evicted by cleanOld")
}

func TestChatController_CleanOldExitsOnCtx(t *testing.T) {
	defer withShortTickers(t)()

	ctx, cancel := context.WithCancel(context.Background())
	c := chatController{userInWork: map[int64]time.Time{}, mux: &sync.Mutex{}}

	done := make(chan struct{})
	go func() {
		c.cleanOld(ctx)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanOld did not exit on ctx cancel")
	}
}

// --- LoaderButton timeout & edit branches ----------------------------------

func TestLoaderButton_DebugTimeoutPostsTimeoutMessage(t *testing.T) {
	defer withShortTickers(t)()

	bot, mock := newTestBot()
	bot.debug = true

	cancel := bot.LoaderButton(1, []string{"a", "b"})
	defer cancel()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mock.mu.Lock()
		var sawTimeout bool
		for _, m := range mock.sent {
			if edit, ok := m.(tgbotapi.EditMessageTextConfig); ok && edit.Text == "Load screen timeout" {
				sawTimeout = true
				break
			}
		}
		mock.mu.Unlock()
		if sawTimeout {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timeout message not produced after maxLoaderTicks")
}

func TestLoaderButton_MultiScreenEditsMessage(t *testing.T) {
	defer withShortTickers(t)()

	bot, mock := newTestBot()
	cancel := bot.LoaderButton(1, []string{"frame1", "frame2"})

	// Wait for at least one edit message.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mock.mu.Lock()
		var sawEdit bool
		for _, m := range mock.sent {
			if _, ok := m.(tgbotapi.EditMessageTextConfig); ok {
				sawEdit = true
				break
			}
		}
		mock.mu.Unlock()
		if sawEdit {
			cancel()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	t.Fatal("expected an EditMessage call from the loader animation")
}

// --- ChatBotImpl.RegisterIButton wraps default layer ------------------------

func TestChatBotRegisterIButton_AddsToDefaultLayer(t *testing.T) {
	bot, _ := newTestBot()
	bot.RegisterIButton("OK", func(_ context.Context, _ Event) error { return nil })

	if len(bot.defaultHandlerLayer.buttonHandler) != 1 {
		t.Fatalf("expected 1 inline button on default layer, got %d",
			len(bot.defaultHandlerLayer.buttonHandler))
	}
}

// --- findAndWipeChatLayerHandler happy path --------------------------------

func TestFindAndWipeChatLayerHandler(t *testing.T) {
	bot, _ := newTestBot()

	// No chat-specific layer → returns default.
	if got := bot.findAndWipeChatLayerHandler(123); got != bot.defaultHandlerLayer {
		t.Fatal("expected default layer fallback")
	}

	// Chat-specific layer is consumed.
	custom := bot.NewLayer()
	bot.setLayer(custom, 5)
	if got := bot.findAndWipeChatLayerHandler(5); got != custom {
		t.Fatal("expected chat-specific layer")
	}
	// And subsequent lookup returns default again.
	if got := bot.findAndWipeChatLayerHandler(5); got != bot.defaultHandlerLayer {
		t.Fatal("layer not wiped after lookup")
	}
}

// --- lookupCallbackButtonText: nil CallbackData branch ---------------------

func TestLookupCallbackButtonText_SkipsNilCallbackData(t *testing.T) {
	other := "other"
	q := &tgbotapi.CallbackQuery{
		Data: "want",
		Message: &tgbotapi.Message{
			ReplyMarkup: &tgbotapi.InlineKeyboardMarkup{
				InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
					{
						{Text: "no-data"},                          // CallbackData == nil
						{Text: "other-data", CallbackData: &other}, // mismatch
					},
				},
			},
		},
	}
	if got := lookupCallbackButtonText(q); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

func TestLookupCallbackButtonText_NilQuery(t *testing.T) {
	if got := lookupCallbackButtonText(nil); got != "" {
		t.Fatalf("want empty for nil query, got %q", got)
	}
}

// --- noopLogger direct calls (forces statements to be executed) ------------

func TestNoopLogger_Direct(t *testing.T) {
	l := noopLogger{}
	l.Debug("a")
	l.Debugf("%s", "a")
	l.Info("a")
	l.Infof("%s", "a")
	l.Warn("a")
	l.Warnf("%s", "a")
	l.Error("a")
	l.Errorf("%s", "a")
}

// --- Start path with mock --------------------------------------------------

func TestStart_RunsAndStopsOnContextCancel(t *testing.T) {
	bot, mock := newTestBot()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- bot.Start(ctx) }()

	// Give Start a moment to subscribe to updates.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return after context cancel")
	}
	if !mock.stopped.Load() {
		t.Fatal("Start did not call StopReceivingUpdates on shutdown")
	}
}

func TestStart_NilTelegramClient(t *testing.T) {
	bot, _ := newTestBot()
	bot.tgbot = nil
	if err := bot.Start(context.Background()); err == nil {
		t.Fatal("expected error when tgbot is nil")
	}
}

// --- RegisterMiddleware concurrency safety ---------------------------------

func TestRegisterMiddleware_ConcurrentWithDispatch(t *testing.T) {
	bot, _ := newTestBot()

	var hits atomic.Int32
	final := func(_ context.Context, _ Event) error { hits.Add(1); return nil }

	stop := make(chan struct{})

	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				bot.RegisterMiddleware(func(next HandlerFunc) HandlerFunc { return next })
			}
		}
	}()

	for i := 0; i < 200; i++ {
		_ = bot.applyMiddlewares(final)(context.Background(), Event{})
	}
	close(stop)

	if hits.Load() != 200 {
		t.Fatalf("dispatch saw %d, want 200", hits.Load())
	}
}

// --- Stop is safe before Start --------------------------------------------

func TestStop_BeforeStart(t *testing.T) {
	bot, _ := newTestBot()
	bot.Stop() // must not panic
}

// --- realTelegramAPI adapter wraps tgbotapi.BotAPI -------------------------

// stubTGServer fakes the minimum endpoints used by tgbotapi.NewBotAPI and
// the methods we delegate through realTelegramAPI. The Telegram API path
// shape is /bot<token>/<method>, so we route by suffix.
func stubTGServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Stub","username":"stub_bot"}}`))
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1,"type":"private"}}}`))
		case strings.HasSuffix(r.URL.Path, "/getFile"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"file_id":"f","file_unique_id":"u","file_path":"path/to/f"}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		}
	}))
}

func newRealTelegramAPI(t *testing.T) (*realTelegramAPI, *httptest.Server) {
	t.Helper()
	srv := stubTGServer(t)
	api, err := tgbotapi.NewBotAPIWithAPIEndpoint("token", srv.URL+"/bot%s/%s")
	if err != nil {
		srv.Close()
		t.Fatalf("create stub bot api: %v", err)
	}
	return &realTelegramAPI{bot: api}, srv
}

func TestRealTelegramAPI_SendAndSelf(t *testing.T) {
	api, srv := newRealTelegramAPI(t)
	defer srv.Close()

	if api.Self().UserName != "stub_bot" {
		t.Fatalf("Self.UserName=%q", api.Self().UserName)
	}

	if _, err := api.Send(tgbotapi.NewMessage(1, "hi")); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestRealTelegramAPI_GetFileDirectURL(t *testing.T) {
	api, srv := newRealTelegramAPI(t)
	defer srv.Close()

	url, err := api.GetFileDirectURL("f")
	if err != nil {
		t.Fatalf("GetFileDirectURL: %v", err)
	}
	if !strings.Contains(url, "path/to/f") {
		t.Fatalf("unexpected URL: %q", url)
	}
}

func TestRealTelegramAPI_StopReceivingUpdates(t *testing.T) {
	api, srv := newRealTelegramAPI(t)
	defer srv.Close()
	api.StopReceivingUpdates() // must not panic and must be idempotent.
	api.StopReceivingUpdates()
}

func TestRealTelegramAPI_GetUpdatesChan(t *testing.T) {
	api, srv := newRealTelegramAPI(t)
	defer srv.Close()

	ch := api.GetUpdatesChan(tgbotapi.UpdateConfig{Timeout: 0})
	api.StopReceivingUpdates()

	// Drain — channel should eventually close after StopReceivingUpdates.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("updates channel did not close after StopReceivingUpdates")
		}
	}
}

// --- NewBotWithEndpoint success path via stub server ------------------------

func TestNewBotWithEndpoint_Success(t *testing.T) {
	srv := stubTGServer(t)
	defer srv.Close()

	bot, err := NewBotWithEndpoint("token", srv.URL+"/bot%s/%s", WithDebug())
	if err != nil {
		t.Fatalf("NewBotWithEndpoint: %v", err)
	}
	defer bot.Stop()

	if !bot.debug {
		t.Fatal("WithDebug option not applied")
	}
	if bot.defaultHandlerLayer == nil {
		t.Fatal("default layer not initialised")
	}
	if bot.errorHandler == nil {
		t.Fatal("default error handler not registered")
	}

	if err := bot.SendText(1, "hi"); err != nil {
		t.Fatalf("SendText via NewBotWithEndpoint: %v", err)
	}
	if bot.SelfUserName() != "stub_bot" {
		t.Fatalf("SelfUserName=%q", bot.SelfUserName())
	}
	if _, err := bot.GetFileURL("f"); err != nil {
		t.Fatalf("GetFileURL: %v", err)
	}
}

func TestNewBotWithEndpoint_BadEndpoint(t *testing.T) {
	if _, err := NewBotWithEndpoint("token", "http://127.0.0.1:1/missing/%s/%s"); err == nil {
		t.Fatal("expected error for unreachable endpoint")
	}
}

// --- LoaderButton: initial Send error logged but loop still cancels --------

func TestLoaderButton_InitialSendError(t *testing.T) {
	defer withShortTickers(t)()

	bot, mock := newTestBot()
	mock.sendErr = errors.New("send fail") // affects every Send

	cancel := bot.LoaderButton(1, []string{"a"})
	time.Sleep(20 * time.Millisecond)
	cancel()
}

// --- loaderButtonLoop: edit-message error path -----------------------------

func TestLoaderButton_EditError(t *testing.T) {
	defer withShortTickers(t)()

	bot, mock := newTestBot()

	// Cause Send to succeed on the first call (initial message) but fail on
	// subsequent edits, exercising the "failed to send loader message" branch.
	var calls atomic.Int32
	mock.mu.Lock()
	mock.sendErr = nil
	mock.mu.Unlock()

	// Wrap mock by replacing tgbot with a small forwarder.
	bot.tgbot = errOnEditAPI{inner: mock, calls: &calls}

	cancel := bot.LoaderButton(1, []string{"a", "b"})
	time.Sleep(50 * time.Millisecond)
	cancel()
}

type errOnEditAPI struct {
	inner *mockTelegramAPI
	calls *atomic.Int32
}

func (e errOnEditAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	n := e.calls.Add(1)
	if _, isEdit := c.(tgbotapi.EditMessageTextConfig); isEdit && n > 1 {
		return tgbotapi.Message{}, errors.New("edit failed")
	}
	return e.inner.Send(c)
}
func (e errOnEditAPI) GetUpdatesChan(cfg tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return e.inner.GetUpdatesChan(cfg)
}
func (e errOnEditAPI) GetFileDirectURL(s string) (string, error) { return e.inner.GetFileDirectURL(s) }
func (e errOnEditAPI) StopReceivingUpdates()                     { e.inner.StopReceivingUpdates() }
func (e errOnEditAPI) Self() tgbotapi.User                       { return e.inner.Self() }

// --- handleUpdate: handler error invokes errorHandler ----------------------

func TestHandleUpdate_HandlerErrorReachesErrorHandler(t *testing.T) {
	bot, _ := newTestBot()

	var captured atomic.Bool
	bot.RegisterErrorHandler(func(_ context.Context, _ Event, err error) {
		if err != nil {
			captured.Store(true)
		}
	})
	bot.RegisterCommand("/fail", func(_ context.Context, _ Event) error {
		return errors.New("boom")
	})

	c := newChatController(context.Background())
	bot.handleUpdate(context.Background(), c, tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text:     "/fail",
			Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 5}},
			Chat:     &tgbotapi.Chat{ID: 1},
			From:     &tgbotapi.User{ID: 1},
		},
	})

	if !captured.Load() {
		t.Fatal("error handler not invoked when handler returned an error")
	}
}
