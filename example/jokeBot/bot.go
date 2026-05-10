package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/smoreg/bf"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tgBotKey := os.Getenv("TEST_TGBOT_API_KEY")
	if tgBotKey == "" {
		logger.Error("TEST_TGBOT_API_KEY is not set")
		os.Exit(1)
	}

	bot, err := bf.NewBot(tgBotKey, bf.WithLogger(slogAdapter{logger}))
	if err != nil {
		logger.Error("failed to create bot", slog.Any("err", err))
		os.Exit(1)
	}
	defer bot.Stop()

	srv := Service{bot, fakeJokeRepo{}}
	bot.RegisterCommand("/start", srv.start())
	bot.RegisterCommand("/help", srv.help("example for help command"))
	bot.RegisterDefaultHandler(srv.help("unknown action"))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := bot.Start(ctx); err != nil && ctx.Err() == nil {
		logger.Error("bot stopped with error", slog.Any("err", err))
		os.Exit(1)
	}
}
