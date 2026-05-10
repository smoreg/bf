package bf

import "context"

// ChatBot is the high-level interface implemented by ChatBotImpl.
// User code should depend on ChatBot rather than the concrete struct
// where possible — it makes mocking and substitution easier.
type ChatBot interface {
	// Start runs the main update loop. Register all static handlers first.
	// The loop terminates when ctx is cancelled or the updates channel closes.
	Start(ctx context.Context) error

	// Stop releases background goroutines created by NewBot and Start.
	// Safe to call multiple times.
	Stop()

	// SendMsg renders the layer (text + buttons), sends it and installs
	// the layer as the next-message expectation for chatID.
	SendMsg(chatID int64, layer *HandlerLayer) error

	// SendText sends a one-off plain text message without affecting any layer.
	SendText(chatID int64, text string) error

	// RegisterDefaultHandler installs the fallback handler for the default layer.
	RegisterDefaultHandler(handler HandlerFunc)
	// RegisterCommand binds a slash command to a handler on the default layer.
	RegisterCommand(command string, handler HandlerFunc)
	// RegisterIButton adds an inline-keyboard button on the default layer.
	RegisterIButton(btn string, handler HandlerFunc)
	// RegisterButton adds a reply-keyboard button on the default layer.
	RegisterButton(btn string, handler HandlerFunc)
	// RegisterAudio binds a voice-message handler on the default layer.
	RegisterAudio(handler HandlerFunc)

	// RegisterMiddleware appends a middleware applied to every handler.
	RegisterMiddleware(middleware MiddlewareFunc)

	// NewLayer constructs a fresh layer carrying optional message text.
	NewLayer(msgText ...any) *HandlerLayer

	// RetryLastLayer re-sends the layer that was active during the previous
	// message in the chat, optionally overriding the layer text.
	RetryLastLayer(event Event, newText string) error

	// RegisterErrorHandler installs the function called when a handler returns an error.
	RegisterErrorHandler(handler ErrorHandlerFunc)

	// SelfUserName returns the bot's own Telegram username.
	SelfUserName() string

	// LoaderButton sends an animated placeholder; cancel via the returned func.
	LoaderButton(chatID int64, loadScreen []string) context.CancelFunc

	// GetFileURL resolves a Telegram fileID to a directly downloadable URL.
	GetFileURL(fileID string) (string, error)
}
