// Echo is a minimal bf bot that replies to every message with the same text.
// Run with: TEST_TGBOT_API_KEY=<token> go run ./example/echo
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/smoreg/bf"
)

func main() {
	token := os.Getenv("TEST_TGBOT_API_KEY")
	if token == "" {
		log.Fatal("TEST_TGBOT_API_KEY is not set")
	}

	bot, err := bf.NewBot(token)
	if err != nil {
		log.Fatalf("create bot: %v", err)
	}
	defer bot.Stop()

	bot.RegisterDefaultHandler(func(_ context.Context, ev bf.Event) error {
		return bot.SendText(ev.ChatID, fmt.Sprintf("you said: %s", ev.Text))
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := bot.Start(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("bot stopped: %v", err)
	}
}
