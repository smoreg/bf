package main

import (
	"context"

	"github.com/smoreg/bf"
)

var tgBotKey = "your bot key" // CHANGE IT

func main() {
	bot, err := bf.NewBot(tgBotKey, true)
	if err != nil {
		panic(err)
	}
	srv := Service{bot, fakeJokeRepo{}}
	bot.RegisterCommand("/start", srv.start())
	bot.RegisterCommand("/help", srv.help("example for help command"))
	bot.RegisterDefaultHandler(srv.help("unknown action"))
	err = bot.Start(context.Background())
	if err != nil {
		panic(err)
	}
}
