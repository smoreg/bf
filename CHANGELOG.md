# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `NewBotWithEndpoint(apikey, endpoint, opts...)` for self-hosted Telegram Bot
  API servers and integration tests against fake servers.
- `WithUpdateConcurrency(n int)` option caps parallel update processing.
  Default is 256; previously unbounded.
- `Stop()` is now safe before `Start()` and is invoked automatically when
  `Start()` returns.
- `recover()` in `handleUpdate`: a panicking handler is logged with stack
  trace and surfaced through the registered error handler instead of killing
  the process.
- `FEATURES.md` lists the roadmap and known gaps in plain English.

### Changed
- **Breaking**: `NewBot` returns `(nil, err)` on failure instead of a
  partially-initialised bot. Add an `if err != nil` check.
- **Breaking**: `Event.UserTgUsername` removed — it duplicated `Event.Username`.
- **Breaking**: `EventKind` is now an exported type (was unexported `eventType`).
  Existing `event.Kind == bf.EventKindText` comparisons still compile.
- `Logger.*` methods take `any` rather than `interface{}`.
- Default logger is a no-op; `logrus` is no longer a required dependency.
- Minimum Go raised to 1.26.
- `RegisterText` and `RegisterButton` write to separate maps. Reply-keyboard
  buttons take priority over generic text handlers on the same key.
- `chatController` lock TTL (10 min default) is independent of the sweep tick
  (1 min). Long handlers no longer get their lock evicted.
- `mainLoop` uses a bounded semaphore; saturated dispatcher drops the update
  with a log instead of spawning unbounded goroutines.
- `RegisterErrorHandler`, `RegisterDefaultHandler`, `RegisterMiddleware` are
  safe to call concurrently with the dispatcher.
- Background-loop tickers stored in `atomic.Int64` so tests can shorten them
  race-free.
- `WithLayerTTL` rejects non-positive durations with an error log.
- `Event.String()` no longer ends with a trailing newline.
- `chatController.LockChat`/`UnlockChat` renamed to `tryAcquire`/`release`
  with the boolean inverted (`true` = acquired). Both unexported, no external
  break.

### Fixed
- `realTelegramAPI.StopReceivingUpdates` no longer panics when called before
  `GetUpdatesChan`.
- `Start()` now defers `Stop()` so background goroutines actually stop on
  context cancellation.
- `newEvent` rejects updates whose `Message.Chat` is nil.
- `newEvent` callback-query branches are nil-safe for `Message`, `Chat` and
  `ReplyMarkup`.
- `handleUpdate` drops events with no matching handler with a log instead of
  dispatching `nil`.
- `SendMsg(chatID, nil)` returns an error instead of dereferencing a nil layer.
- `LoaderButton` returns immediately on initial-Send failure (was editing
  `MessageID=0` for 40 seconds).
- `LoaderButton` goroutines now terminate when `Stop()` is called.
- `mainLoop` derives a child context for `chatController.cleanOld`, so the
  sweeper terminates when the updates channel closes too.
- `SendText` applies the configured `parseMode`, matching `SendMsg`.
- `IsEmpty` returns false for layers with only a voice handler set.
- Race on `RetryLastLayer` — original layer text is copied before override.
- `NewLayer()` with no message text no longer produces a stray newline.

### Removed
- Dead struct fields: `HandlerLayer.generalMiddlewares`, `Event.UserTgUsername`,
  `TextHandler.id`, `CommandHandler.command`/`id`, `AudioHandler.id`,
  `InlineButtonHandler.text`/`id`.
- Helper test files `noop_logger_test.go`, `test_helpers_test.go`,
  `timeout_test.go` — single-function helpers folded inline.

### Examples
- jokeBot: `Service` holds `*fakeJokeRepo` so `SaveJoke` actually persists;
  `fakeJokeRepo.GetARandomJoke` no longer panics on a non-nil empty slice;
  banned-name demo says "Voldemort" rather than "Hitler".
- echo: switched from `log.Fatal*` to `slog`, exits cleanly on signal.

### CI / lint
- `.golangci.yml` rewritten with an explicit linter allow-list (was
  `enable-all: true`); uses `mnd` (renamed from `gomnd`).
- `.gitleaks.toml` rewritten in v8 schema (was v7-style and ignored).
- CI now fails if `go.mod` / `go.sum` are not tidy.

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
- Unit-test suite, GitHub Actions CI, golangci-lint and gitleaks pipelines,
  dependabot, GoReleaser-driven tagged releases.
