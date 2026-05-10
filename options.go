package bf

import "time"

// BotOption configures a ChatBotImpl during NewBot.
type BotOption func(bot *ChatBotImpl)

// WithDebug enables debug-mode behaviour: verbose logging and the default
// event handler echoing unmatched events back to the chat as JSON.
func WithDebug() BotOption {
	return func(bot *ChatBotImpl) {
		bot.debug = true
	}
}

// WithLogger replaces the default no-op logger with the provided implementation.
// Pass any Logger (logrus, zap, slog wrapper, custom).
func WithLogger(logger Logger) BotOption {
	return func(bot *ChatBotImpl) {
		if logger != nil {
			bot.logger = logger
		}
	}
}

// WithParseMode sets the parse mode applied to messages sent via SendMsg.
// Accepts tgbotapi.ModeMarkdown, ModeMarkdownV2 or ModeHTML (default).
func WithParseMode(parseMode string) BotOption {
	return func(bot *ChatBotImpl) {
		bot.parseMode = parseMode
	}
}

// WithLayerTTL overrides how long a chat-specific layer stays active before
// being garbage-collected by the cleaner. Default is 24 hours.
func WithLayerTTL(d time.Duration) BotOption {
	return func(bot *ChatBotImpl) {
		if d > 0 {
			bot.defaultTTL = d
		}
	}
}
