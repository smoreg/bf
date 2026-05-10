<img alt="bf mascot" align="right" width="240" height="240" src="https://i.ibb.co/P69qSRC/smoreg-Combine-the-image-of-the-Golang-Gopher-and-Cthulhu-into-59680cfc-4566-4e6b-bbfb-791d2dd37d94.png">

# bf — bot framework for Telegram

[![CI](https://github.com/smoreg/bf/actions/workflows/ci.yml/badge.svg)](https://github.com/smoreg/bf/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/smoreg/bf/branch/main/graph/badge.svg)](https://codecov.io/gh/smoreg/bf)
[![Go Reference](https://pkg.go.dev/badge/github.com/smoreg/bf.svg)](https://pkg.go.dev/github.com/smoreg/bf)
[![Go Report Card](https://goreportcard.com/badge/github.com/smoreg/bf)](https://goreportcard.com/report/github.com/smoreg/bf)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/smoreg/bf)](go.mod)

A small Go framework for writing Telegram bots around a per-chat *layer*
pattern: each `SendMsg` installs a one-shot set of handlers expected from the
user's next message, with a permanent default layer as fallback.

> **Status:** alpha. The public API is usable but may evolve before `v1.0.0` —
> pin a tag in your `go.mod`.

## Install

```sh
go get github.com/smoreg/bf
```

Requires Go 1.26 or newer.

## Quick start

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/smoreg/bf"
)

func main() {
    bot, err := bf.NewBot(os.Getenv("TG_TOKEN"))
    if err != nil {
        log.Fatal(err)
    }
    defer bot.Stop()

    bot.RegisterCommand("/start", func(ctx context.Context, ev bf.Event) error {
        return bot.SendText(ev.ChatID, "Hello, "+ev.FirstName)
    })

    if err := bot.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

## Core concepts

- **`ChatBot`** — the bot interface. Construct with `bf.NewBot(token, opts...)`.
- **`HandlerLayer`** — a set of handlers (commands, text, inline buttons,
  reply-keyboard buttons, voice). Build one via `bot.NewLayer(...)` and send
  it with `bot.SendMsg(chatID, layer)` — the layer matches the very next
  message from that chat, then is wiped. Handlers registered directly on the
  bot live on a permanent default layer.
- **Middlewares** — wrap every handler. Registered via
  `bot.RegisterMiddleware(...)`; applied in registration order, last added
  runs outermost.
- **Options** — `WithLogger`, `WithDebug`, `WithParseMode`, `WithLayerTTL`.
  By default the bot uses a no-op logger and HTML parse mode.

## Examples

- [`example/echo`](example/echo) — minimal echo bot.
- [`example/jokeBot`](example/jokeBot) — multi-step dialogue with inline
  buttons, reply text and `RetryLastLayer`.

Run an example with:

```sh
TEST_TGBOT_API_KEY=<your-bot-token> go run ./example/echo
```

## Roadmap

Tracked in [GitHub issues](https://github.com/smoreg/bf/issues). Open one if a
feature you need is missing.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for local
setup and the PR checklist. By participating you agree to abide by the
[Code of Conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE)
