package bf

import "time"

type eventType string

// Recognised event kinds emitted by newEvent.
const (
	EventKindText         eventType = "text"
	EventKindInlineButton eventType = "buttonInline"
	EventKindCommand      eventType = "command"
	EventKindVoice        eventType = "audio"
)

// loaderTickDelay is the cadence at which LoaderButton refreshes its placeholder
// message. Var (not const) so tests can drive the loop quickly.
var loaderTickDelay = 2 * time.Second
