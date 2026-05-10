package bf

import (
	"sync/atomic"
	"time"
)

// EventKind identifies the kind of payload an Event carries.
type EventKind string

// Recognised event kinds emitted by newEvent.
const (
	EventKindText         EventKind = "text"
	EventKindInlineButton EventKind = "buttonInline"
	EventKindCommand      EventKind = "command"
	EventKindVoice        EventKind = "audio"
)

// Loader timing.
//
// loaderTickDelay is the cadence at which LoaderButton refreshes its
// placeholder message. Atomic so tests can drive the loop quickly without
// racing the goroutines that read it.
var loaderTickDelay atomic.Int64

func init() {
	loaderTickDelay.Store(int64(2 * time.Second))
}

func loaderTick() time.Duration { return time.Duration(loaderTickDelay.Load()) }
