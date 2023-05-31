package bf

import "context"

type (
	HandlerFunc      func(ctx context.Context, event Event) error
	MiddlewareFunc   func(handlerFunc HandlerFunc) HandlerFunc
	ErrorHandlerFunc func(context.Context, Event, error)
	FilterFunc       func(ctx context.Context, event Event) bool
)
