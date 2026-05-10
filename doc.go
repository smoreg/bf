// Package bf is a small framework for building Telegram bots in Go.
//
// The core abstraction is a [HandlerLayer] — a set of handlers expected from
// the next message a particular chat sends. A bot has one always-present
// default layer and may install short-lived per-chat layers via [ChatBot.SendMsg].
// When the next event arrives from that chat, the per-chat layer is consumed
// and the bot falls back to the default layer afterwards.
//
// Quick start:
//
//	bot, err := bf.NewBot(os.Getenv("TG_TOKEN"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer bot.Stop()
//
//	bot.RegisterCommand("/start", func(ctx context.Context, ev bf.Event) error {
//	    return bot.SendText(ev.ChatID, "Hello, "+ev.FirstName)
//	})
//
//	if err := bot.Start(context.Background()); err != nil {
//	    log.Fatal(err)
//	}
//
// See the example/ subdirectories for complete bots.
package bf
