package bf

type eventType string

const (
	EventKindText         eventType = "text"
	EventKindInlineButton eventType = "buttonInline"
	EventKindCommand      eventType = "command"
	EventKindVoice        eventType = "audio"
	loaderTickDelay                 = 2000
)
