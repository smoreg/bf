package main

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/smoreg/bf"
)

func main() {
	tgBotKey := os.Getenv("TEST_TGBOT_API_KEY")

	bot, err := bf.NewBot(tgBotKey)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create bot")
	}

	srv := Service{bot, fakeJokeRepo{}}
	bot.RegisterCommand("/start", srv.start())
	bot.RegisterCommand("/help", srv.help("example for help command"))
	bot.RegisterDefaultHandler(srv.help("unknown action"))

	err = bot.Start(context.Background())
	if err != nil {
		logrus.WithError(err).Fatal("failed to start bot")
	}
}
