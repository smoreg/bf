package bf

type BotOption func(bot *ChatBotImpl)

// WithDebug enables debug mode.
func WithDebug() BotOption {
	return func(bot *ChatBotImpl) {
		bot.debug = true
	}
}

// WithLogger inject your logger inside framework.
func WithLogger(logger Logger) BotOption {
	return func(bot *ChatBotImpl) {
		bot.logger = logger
	}
}

// WithParseMode sets parse mode for TG for all messages.
// default is ModeHTML
// Possible values:
//   - ModeMarkdown
//   - ModeMarkdownV2
//   - ModeHTML
func WithParseMode(parseMode string) BotOption {
	return func(bot *ChatBotImpl) {
		bot.parseMode = parseMode
	}
}
