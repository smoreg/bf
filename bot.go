package bf

import (
	"fmt"
	"sync"
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
// It exists because BotAPI exposes Self as a struct field, not a method.
type realTelegramAPI struct {
	bot *tgbotapi.BotAPI
}

func (r realTelegramAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return r.bot.Send(c)
}

func (r realTelegramAPI) GetUpdatesChan(cfg tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return r.bot.GetUpdatesChan(cfg)
}

func (r realTelegramAPI) GetFileDirectURL(fileID string) (string, error) {
	return r.bot.GetFileDirectURL(fileID)
}

func (r realTelegramAPI) StopReceivingUpdates() { r.bot.StopReceivingUpdates() }
func (r realTelegramAPI) Self() tgbotapi.User   { return r.bot.Self }

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

	middlewares  []MiddlewareFunc
	errorHandler ErrorHandlerFunc
	logger       Logger

	debug      bool
	parseMode  string
	defaultTTL time.Duration

	// shutdownOnce guards Stop so the cleaner channel is closed exactly once.
	shutdownOnce sync.Once
	shutdown     chan struct{}
}

// NewBot creates a new bot bound to the given Telegram bot API key.
// Pass functional options (WithLogger, WithDebug, WithParseMode, WithLayerTTL) to customise.
// The returned bot is not started; call Start to begin processing updates.
func NewBot(apikey string, opts ...BotOption) (*ChatBotImpl, error) {
	chatBot := &ChatBotImpl{
		chatHandlerLayers:   make(map[int64]*HandlerLayer),
		defaultHandlerLayer: nil,
		middlewares:         make([]MiddlewareFunc, 0),
		errorHandler:        nil,
		logger:              noopLogger{},
		debug:               false,
		parseMode:           tgbotapi.ModeHTML,
		defaultTTL:          24 * time.Hour,
		shutdown:            make(chan struct{}),
	}
	for _, opt := range opts {
		opt(chatBot)
	}

	bot, err := tgbotapi.NewBotAPI(apikey)
	if err != nil {
		return chatBot, fmt.Errorf("failed to create bot: %w", err)
	}

	chatBot.tgbot = realTelegramAPI{bot: bot}

	chatBot.defaultHandlerLayer = chatBot.NewLayer()
	chatBot.defaultHandlerLayer.ttl = time.Now().Add(layerTTLForever)
	chatBot.RegisterErrorHandler(chatBot.defaultErrorHandler)
	chatBot.RegisterDefaultHandler(chatBot.defaultEventHandler)

	go chatBot.cleaner()

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
