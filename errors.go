package bf

import "errors"

// ErrUnparsedEvent is returned when an incoming Telegram update cannot be
// translated into an Event the framework understands.
var ErrUnparsedEvent = errors.New("unparsed event")
