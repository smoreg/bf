package bf

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// telegramAPI is the narrow surface of github.com/go-telegram-bot-api/telegram-bot-api/v5
// that the framework actually uses. Extracting it keeps the bot unit-testable
// without requiring a real Telegram connection.
type telegramAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	GetUpdatesChan(cfg tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	GetFileDirectURL(fileID string) (string, error)
	StopReceivingUpdates()
	Self() tgbotapi.User
}

// realTelegramAPI adapts *tgbotapi.BotAPI to the telegramAPI interface.
// It exists because BotAPI exposes Self as a struct field, not a method, and
// because BotAPI.StopReceivingUpdates panics if GetUpdatesChan has not been
// called yet — we need to guard against that for safe defer-Stop semantics.
type realTelegramAPI struct {
	bot      *tgbotapi.BotAPI
	updating atomic.Bool
}

func (r *realTelegramAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return r.bot.Send(c)
}

func (r *realTelegramAPI) GetUpdatesChan(cfg tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	r.updating.Store(true)
	return r.bot.GetUpdatesChan(cfg)
}

func (r *realTelegramAPI) GetFileDirectURL(fileID string) (string, error) {
	return r.bot.GetFileDirectURL(fileID)
}

// StopReceivingUpdates is a no-op if GetUpdatesChan was never invoked, since
// the underlying BotAPI panics on a closed-channel double-close in that case.
func (r *realTelegramAPI) StopReceivingUpdates() {
	if r.updating.CompareAndSwap(true, false) {
		r.bot.StopReceivingUpdates()
	}
}

func (r *realTelegramAPI) Self() tgbotapi.User { return r.bot.Self }

// layerTTLForever marks the default layer as effectively non-expiring.
const layerTTLForever = time.Hour * 24 * 365 * 100

var _ ChatBot = &ChatBotImpl{}

// ChatBotImpl is the default ChatBot implementation. Construct via NewBot.
type ChatBotImpl struct {
	tgbot telegramAPI

	// chatHandlerLayers maps a chat id to a per-chat layer of one-time handlers
	// installed by the previous SendMsg. Cleared on the next received message.
	chatHandlerLayers map[int64]*HandlerLayer
	// defaultHandlerLayer is the always-present fallback layer used when no
	// chat-specific layer matches. Never wiped automatically.
	defaultHandlerLayer *HandlerLayer

	layersMutex sync.RWMutex
	// defaultLayerMutex guards mutations on defaultHandlerLayer (the
	// Register* methods that mutate handler maps, and reads of those maps
	// from the dispatcher). Per-chat layers are short-lived and not shared
	// across goroutines after SendMsg, so they need no separate lock.
	defaultLayerMutex sync.RWMutex

	middlewaresMutex sync.RWMutex
	middlewares      []MiddlewareFunc

	errorHandlerMutex sync.RWMutex
	errorHandler      ErrorHandlerFunc

	logger Logger

	debug             bool
	parseMode         string
	defaultTTL        time.Duration
	updateConcurrency int

	// shutdownOnce guards Stop so the cleaner channel is closed exactly once.
	shutdownOnce sync.Once
	shutdown     chan struct{}
}

// getErrorHandler returns the currently registered error handler under a read lock.
func (b *ChatBotImpl) getErrorHandler() ErrorHandlerFunc {
	b.errorHandlerMutex.RLock()
	defer b.errorHandlerMutex.RUnlock()
	return b.errorHandler
}

// newSkeleton builds an unwired ChatBotImpl with options applied. The caller
// is responsible for attaching a telegramAPI and starting background goroutines.
func newSkeleton(opts []BotOption) *ChatBotImpl {
	chatBot := &ChatBotImpl{
		chatHandlerLayers:   make(map[int64]*HandlerLayer),
		defaultHandlerLayer: nil,
		middlewares:         make([]MiddlewareFunc, 0),
		errorHandler:        nil,
		logger:              noopLogger{},
		debug:               false,
		parseMode:           tgbotapi.ModeHTML,
		defaultTTL:          24 * time.Hour,
		updateConcurrency:   defaultUpdateConcurrency,
		shutdown:            make(chan struct{}),
	}
	for _, opt := range opts {
		opt(chatBot)
	}
	if chatBot.updateConcurrency <= 0 {
		chatBot.updateConcurrency = defaultUpdateConcurrency
	}
	return chatBot
}

// finalise wires defaults and starts background goroutines after a telegramAPI
// has been attached. Shared by NewBot and NewBotWithEndpoint.
func (b *ChatBotImpl) finalise() {
	b.defaultHandlerLayer = b.NewLayer()
	b.defaultHandlerLayer.ttl = time.Now().Add(layerTTLForever)
	b.RegisterErrorHandler(b.defaultErrorHandler)
	b.RegisterDefaultHandler(b.defaultEventHandler)

	go b.cleaner()
}

// NewBot creates a new bot bound to the given Telegram bot API key, talking to
// the public Telegram Bot API. Pass functional options (WithLogger, WithDebug,
// WithParseMode, WithLayerTTL) to customise. The returned bot is not started;
// call Start to begin processing updates.
//
// On error the returned *ChatBotImpl is nil — always check err.
func NewBot(apikey string, opts ...BotOption) (*ChatBotImpl, error) {
	bot, err := tgbotapi.NewBotAPI(apikey)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	chatBot := newSkeleton(opts)
	chatBot.tgbot = &realTelegramAPI{bot: bot}
	chatBot.finalise()
	return chatBot, nil
}

// NewBotWithEndpoint behaves like NewBot but routes API calls through the
// supplied endpoint template. The template must contain two %s placeholders
// (one for the token, one for the method) — see tgbotapi.APIEndpoint for the
// canonical format. Useful for self-hosted Telegram Bot API servers and for
// integration tests against a fake server.
func NewBotWithEndpoint(apikey, endpoint string, opts ...BotOption) (*ChatBotImpl, error) {
	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint(apikey, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	chatBot := newSkeleton(opts)
	chatBot.tgbot = &realTelegramAPI{bot: bot}
	chatBot.finalise()
	return chatBot, nil
}

// Stop releases background goroutines created by NewBot and Start.
// Safe to call multiple times. After Stop the bot must not be reused.
func (b *ChatBotImpl) Stop() {
	b.shutdownOnce.Do(func() {
		close(b.shutdown)
		if b.tgbot != nil {
			b.tgbot.StopReceivingUpdates()
		}
	})
}
