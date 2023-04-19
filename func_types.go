package bf

import "context"

type HandlerFunc func(ctx context.Context, event Event) error
type MiddlewareFunc func(handlerFunc HandlerFunc) HandlerFunc
type ErrorHandlerFunc func(context.Context, Event, error)
type FilterFunc func(ctx context.Context, event Event) bool
