package bf

import (
	"context"
)

type BotBuilder interface {
	// Start main loop of the bot. Start after register all non-dynamic handlers.
	Start(context.Context) error
	// SendMsg sends a message to the chat. Buttons register one-time handlers.
	SendMsg(chatID int64, layer *HandlerLayer) error
	// SendTxt sends a short text message to the chat. Doesn't wipe layers.
	SendText(chatID int64, text string) error

	// RegisterDefaultHandler registers a handler that will be called if no other handler is found.
	RegisterDefaultHandler(handler HandlerFunc)
	// RegisterHandlerCommand registers a handler for a command.
	RegisterCommand(command string, handler HandlerFunc)
	RegisterIButton(btn string, handler HandlerFunc)
	// RegisterMiddleware middlewares before any handler that matches the filter function.
	// If the filter function returns true, the middleware will be applied.
	// If filterFunc is nil, the middleware will be applied to all handlers.
	RegisterMiddleware(middleware MiddlewareFunc)

	// NewLayer creates a new Layer of handlers necessary for SendMsg.
	NewLayer() *HandlerLayer

	// RetryLastLayer SendMsg with the same layer as the last one.
	RetryLastLayer(event Event, newText string) error

	// RegisterErrorHandler sets the default handler if error happens on handler.
	RegisterErrorHandler(handler ErrorHandlerFunc)
	// SelfUserName returns the username of the bot.
	SelfUserName() string
}
