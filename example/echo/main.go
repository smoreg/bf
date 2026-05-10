// Echo is a minimal bf bot that replies to every message with the same text.
// Run with: TEST_TGBOT_API_KEY=<token> go run ./example/echo
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/smoreg/bf"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	token := os.Getenv("TEST_TGBOT_API_KEY")
	if token == "" {
		logger.Error("TEST_TGBOT_API_KEY is not set")
		os.Exit(1)
	}

	bot, err := bf.NewBot(token)
	if err != nil {
		logger.Error("create bot", slog.Any("err", err))
		os.Exit(1)
	}
	defer bot.Stop()

	bot.RegisterDefaultHandler(func(_ context.Context, ev bf.Event) error {
		return bot.SendText(ev.ChatID, fmt.Sprintf("you said: %s", ev.Text))
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := bot.Start(ctx); err != nil && ctx.Err() == nil {
		logger.Error("bot stopped", slog.Any("err", err))
		cancel()
		bot.Stop()
		// nolint:gocritic // we have already run all defers we care about
		os.Exit(1)
	}
}
