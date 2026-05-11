<img alt="bf mascot" align="right" width="220" height="220" src="https://i.ibb.co/P69qSRC/smoreg-Combine-the-image-of-the-Golang-Gopher-and-Cthulhu-into-59680cfc-4566-4e6b-bbfb-791d2dd37d94.png">

# bf

[![CI](https://github.com/smoreg/bf/actions/workflows/ci.yml/badge.svg)](https://github.com/smoreg/bf/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/smoreg/bf/branch/main/graph/badge.svg)](https://codecov.io/gh/smoreg/bf)
[![Go Reference](https://pkg.go.dev/badge/github.com/smoreg/bf.svg)](https://pkg.go.dev/github.com/smoreg/bf)
[![Go Report Card](https://goreportcard.com/badge/github.com/smoreg/bf)](https://goreportcard.com/report/github.com/smoreg/bf)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A small, opinionated Telegram-bot framework for Go.

## Install

```sh
go get github.com/smoreg/bf
```

Requires Go 1.26.

## Hello, world

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

    bot.RegisterCommand("/start", func(_ context.Context, ev bf.Event) error {
        return bot.SendText(ev.ChatID, "hi, "+ev.FirstName)
    })

    if err := bot.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

That's it. Ctrl-C to stop.

## How it works

You register **handlers** on the bot. A handler is `func(ctx, event) error`.

* `RegisterCommand("/foo", h)` — fires when the user types `/foo`.
* `RegisterDefaultHandler(h)` — fires for anything else.
* Inside any handler you can build a **layer** with `bot.NewLayer("ask")`,
  attach buttons via `layer.RegisterIButton("Yes", h)`, and send it with
  `bot.SendMsg(chatID, layer)`. The layer matches the **next** message from
  that chat, then is wiped.

That's the whole model: permanent default handlers + per-chat one-shot
layers for follow-up questions.

## Options

```go
bot, _ := bf.NewBot(token,
    bf.WithLogger(myLogger),         // any 8-method logger; default is no-op
    bf.WithParseMode(tgbotapi.ModeHTML),
    bf.WithLayerTTL(30 * time.Minute),
    bf.WithUpdateConcurrency(64),
)
```

For a self-hosted Telegram Bot API server use `bf.NewBotWithEndpoint(token, endpoint, opts...)`.

## Examples

* [`example/echo`](example/echo) — minimal echo bot.
* [`example/jokeBot`](example/jokeBot) — multi-step dialogue with inline
  buttons and `RetryLastLayer`.

```sh
TEST_TGBOT_API_KEY=<your-bot-token> go run ./example/echo
```

## Status

Alpha — the API may change before `v1.0.0`. Pin a tag in your `go.mod`.
Roadmap and known gaps are in [FEATURES.md](FEATURES.md).

## Contributing

PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md).
For security issues please follow [SECURITY.md](SECURITY.md).

## License

MIT — see [LICENSE](LICENSE).
