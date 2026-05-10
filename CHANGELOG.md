# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `NewBotWithEndpoint(apikey, endpoint, opts...)` for self-hosted Telegram Bot
  API servers and integration tests against fake servers.
- `Stop()` is now safe to call before `Start()` and is invoked automatically
  when `Start()` returns, so callers no longer have to remember to defer it.
- `WithUpdateConcurrency(n int)` option caps how many updates the dispatcher
  processes in parallel. Without it the bot now defaults to 256 (was unbounded).
- Audit follow-ups, second batch:
  - `recover()` in `handleUpdate`: a panicking handler is logged with stack
    trace and surfaced through the registered error handler instead of
    killing the process.
  - `RegisterErrorHandler` and `RegisterDefaultHandler` are now safe to call
    concurrently with the dispatcher.
  - `chatController` lock TTL (10 min default) is now independent of the
    sweep tick (1 min default), so a long-running handler no longer has its
    lock evicted by the sweeper while it is still running.
  - `mainLoop` uses a bounded semaphore; under saturation extra updates are
    dropped with a log instead of spawning unbounded goroutines.
  - `RegisterText` and `RegisterButton` no longer share a single map. Reply-
    keyboard buttons live in `buttonTextHandler` and take priority over
    generic text handlers with the same key.
  - `LoaderButton` returns immediately if the initial Send fails, instead of
    spamming `EditMessage(MessageID=0)` for the next 40 seconds.
  - `IsEmpty` now returns false for layers with only a voice handler set.
  - `WithLayerTTL` rejects non-positive values with an error log instead of
    silently keeping the previous default.
  - `Event.String()` no longer ends with a trailing newline.
  - `gitleaks` config rewritten in v8 schema; previous file was syntactically
    invalid for current gitleaks and effectively ignored.
  - `golangci-lint` config moved from `enable-all: true` (broke on every
    linter upgrade) to an explicit allow-list with `mnd` (the renamed
    `gomnd`).
  - CI now fails if `go.mod` / `go.sum` are not tidy.

### Changed
- **Breaking**: `NewBot` now returns `(nil, err)` on failure instead of a
  partially-initialised bot. Callers that ignored the error and used the
  returned struct anyway need to add an `if err != nil` check.
- **Breaking**: `Event.UserTgUsername` removed. It was a duplicate of
  `Event.Username`; both fields used to be populated from the same source.
  Use `Event.Username`.
- **Breaking**: `EventKind` is now an exported type. The `eventType` alias
  is gone. Existing code that compared `event.Kind == bf.EventKindText`
  keeps working unchanged.
- `chatController.LockChat`/`UnlockChat` were renamed to `tryAcquire` /
  `release` (unexported, so not externally breaking) and the boolean was
  inverted: `tryAcquire` returns `true` when it acquired the lock.

### Changed
- **Breaking**: `NewBot` now returns `(nil, err)` on failure instead of a
  partially-initialised bot. Callers that ignored the error and used the
  returned struct anyway need to add an `if err != nil` check.
- `RegisterMiddleware` is now safe to call concurrently with the dispatcher.
- Background-loop tickers are stored in `atomic.Int64` package vars so tests
  can shorten them without racing the goroutines that read them.
- Pruned dead fields (`generalMiddlewares`, unused `id`/`text` on handler
  structs); switched `uuid.UUID` to `uuid.NewString()` to drop a tiny dep edge.

### Fixed
- `realTelegramAPI.StopReceivingUpdates` no longer panics when called before
  `GetUpdatesChan`. This made `Stop()` unsafe in tests and after a failed
  `Start()`.
- `Start()` now defers `Stop()` so background goroutines (`cleaner`) actually
  stop when `Start` returns due to context cancellation.
- `newEvent` rejects updates whose `Message.Chat` is nil instead of panicking.
- `handleUpdate` drops events with no matching handler with a debug log
  instead of dispatching `nil` (which panics inside the middleware chain).
- `SendMsg(chatID, nil)` returns an error rather than dereferencing a nil
  layer.
- `LoaderButton` goroutines now terminate when `Stop()` is called — they
  used to keep running until their individual cancel was invoked.
- `mainLoop` now derives a child context for `chatController.cleanOld`, so
  the sweeper terminates even if the loop exits because the updates channel
  closed (not because ctx was cancelled).

## [0.1.0] - 2026-05-09

### Added
- Initial public release.
- `ChatBot` interface and `ChatBotImpl` implementation around the
  go-telegram-bot-api/v5 client.
- Per-chat `HandlerLayer` with one-shot handlers for text, slash commands,
  inline buttons, reply-keyboard buttons and voice messages.
- Middleware chain via `RegisterMiddleware`.
- Options: `WithDebug`, `WithLogger`, `WithParseMode`, `WithLayerTTL`.
- `LoaderButton` helper for animated long-running operation indicators.
- `RetryLastLayer` to re-send the previously active layer.
- Echo and joke-bot examples.
- Unit-test suite (`84%`+ coverage), GitHub Actions CI, golangci-lint and
  gitleaks pipelines, dependabot, GoReleaser-driven tagged releases.

### Removed
- Dead struct fields: `HandlerLayer.generalMiddlewares`, `Event.UserTgUsername`,
  `TextHandler.id`, `CommandHandler.command`/`id`, `AudioHandler.id`,
  `InlineButtonHandler.text`/`id`.

### Misc
- jokeBot example: `Service` is held by pointer so `SaveJoke` (pointer-
  receiver) actually persists across handler invocations. `fakeJokeRepo`
  guards against panicking on a non-nil empty slice. The "Hitler" string
  in the demo flow was replaced with "Voldemort".

### Changed (continued)
- `Logger` interface now uses `any` instead of `interface{}`.
- Default logger is a no-op; `logrus` is no longer a required dependency of
  the library.
- Minimum Go version raised to 1.26.

### Fixed
- Goroutine leak: cleaner and chat-controller goroutines now stop on `Stop` /
  context cancellation instead of running forever.
- Race in `RetryLastLayer`: the previous layer is copied before its text is
  overridden, so concurrent users of the same layer pointer are unaffected.
- Possible nil-pointer panic in `newEvent` when a callback query carries a
  nil `Message` or `ReplyMarkup`.
- `NewLayer()` with no message text no longer produces a stray newline.
