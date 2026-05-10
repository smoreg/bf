package bf

import (
	"sync"
	"sync/atomic"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// mockTelegramAPI is a hand-rolled, thread-safe stub for telegramAPI used by unit tests.
type mockTelegramAPI struct {
	mu sync.Mutex

	updates chan tgbotapi.Update

	sent     []tgbotapi.Chattable
	sendErr  error
	sendResp tgbotapi.Message

	fileURLs   map[string]string
	fileURLErr error

	self    tgbotapi.User
	stopped atomic.Bool
}

func newMockTelegramAPI() *mockTelegramAPI {
	return &mockTelegramAPI{
		updates:  make(chan tgbotapi.Update, 16),
		fileURLs: map[string]string{},
		self:     tgbotapi.User{UserName: "mock_bot", ID: 1},
	}
}

func (m *mockTelegramAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, c)
	return m.sendResp, m.sendErr
}

func (m *mockTelegramAPI) GetUpdatesChan(_ tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return m.updates
}

func (m *mockTelegramAPI) GetFileDirectURL(fileID string) (string, error) {
	if m.fileURLErr != nil {
		return "", m.fileURLErr
	}
	if v, ok := m.fileURLs[fileID]; ok {
		return v, nil
	}
	return "", nil
}

func (m *mockTelegramAPI) StopReceivingUpdates() { m.stopped.Store(true) }
func (m *mockTelegramAPI) Self() tgbotapi.User   { return m.self }

func (m *mockTelegramAPI) sentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

func (m *mockTelegramAPI) lastSent() tgbotapi.Chattable {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		return nil
	}
	return m.sent[len(m.sent)-1]
}

// newTestBot returns a ChatBotImpl wired to a mock telegram API, with the
// default handler/error handler already installed (mirrors NewBot's tail).
// It bypasses the real tgbotapi.NewBotAPI which requires network.
func newTestBot() (*ChatBotImpl, *mockTelegramAPI) {
	mock := newMockTelegramAPI()
	bot := &ChatBotImpl{
		tgbot:               mock,
		chatHandlerLayers:   make(map[int64]*HandlerLayer),
		defaultHandlerLayer: nil,
		middlewares:         make([]MiddlewareFunc, 0),
		logger:              noopLogger{},
		parseMode:           tgbotapi.ModeHTML,
		defaultTTL:          24 * time.Hour,
		updateConcurrency:   defaultUpdateConcurrency,
		shutdown:            make(chan struct{}),
	}
	bot.defaultHandlerLayer = bot.NewLayer()
	bot.defaultHandlerLayer.ttl = nowPlus(layerTTLForever)
	bot.RegisterErrorHandler(bot.defaultErrorHandler)
	bot.RegisterDefaultHandler(bot.defaultEventHandler)
	return bot, mock
}
