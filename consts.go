package bf

type eventType string

// Recognised event kinds emitted by newEvent.
const (
	EventKindText         eventType = "text"
	EventKindInlineButton eventType = "buttonInline"
	EventKindCommand      eventType = "command"
	EventKindVoice        eventType = "audio"

	loaderTickDelay = 2000
)
