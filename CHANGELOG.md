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

### Changed
- **Breaking**: `NewBot` now returns `(nil, err)` on failure instead of a
  partially-initialised bot. Callers that ignored the error and used the
  returned struct anyway need to add an `if err != nil` check.
- `RegisterMiddleware` is now safe to call concurrently with the dispatcher.
- `cleaner` and `chatController.cleanOld` ticker intervals are package-level
  vars rather than consts so background-loop branches are testable.
- Pruned dead fields (`generalMiddlewares`, unused `id`/`text` on handler
  structs); switched `uuid.UUID` to `uuid.NewString()` to drop a tiny dep edge.

### Fixed
- `realTelegramAPI.StopReceivingUpdates` no longer panics when called before
  `GetUpdatesChan`. This made `Stop()` unsafe in tests and after a failed
  `Start()`.
- `Start()` now defers `Stop()` so background goroutines (`cleaner`) actually
  stop when `Start` returns due to context cancellation.

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

### Changed
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
