package bf

import "context"

// Function-type aliases used throughout the public API.
type (
	// HandlerFunc processes a single normalised event.
	HandlerFunc func(ctx context.Context, event Event) error
	// MiddlewareFunc wraps a HandlerFunc to add cross-cutting behaviour.
	MiddlewareFunc func(handlerFunc HandlerFunc) HandlerFunc
	// ErrorHandlerFunc receives errors returned by handlers.
	ErrorHandlerFunc func(context.Context, Event, error)
	// FilterFunc decides whether a middleware should apply to a given event.
	FilterFunc func(ctx context.Context, event Event) bool
)
