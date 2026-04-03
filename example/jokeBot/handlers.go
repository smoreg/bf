package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/smoreg/bf"
)

type Service struct {
	botFrame bf.ChatBot
	repo     fakeJokeRepo
}

func (s Service) start() bf.HandlerFunc {
	return func(ctx context.Context, event bf.Event) error {
		layer := s.botFrame.NewLayer()
		layer.AddText("Hi! Type or choose you name")
		layer.RegisterIButton("John", s.processName())
		layer.RegisterIButton("Mike", s.processName())
		layer.RegisterIButton("Bob", s.processName())
		layer.RegisterText("Hitler", s.processBannedName())
		layer.RegisterText(bf.AnyText, s.processName())

		err := s.botFrame.SendMsg(event.ChatID, layer)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		return nil
	}
}

func (s Service) help(inp string) bf.HandlerFunc {
	return func(ctx context.Context, event bf.Event) error {
		layer := s.botFrame.NewLayer()
		if inp == "how to write jokes?" {
			layer.AddText("You can write jokes like this:")
			layer.AddText("jokes")
			layer.AddText("Is it obvious?")
			layer.RegisterIButton("Start", s.start())

			err := s.botFrame.SendMsg(event.ChatID, layer)
			if err != nil {
				return fmt.Errorf("failed to send message: %w", err)
			}

			return nil
		}

		layer.AddText("Welcome to help!")
		layer.AddText("There is example help text for \"" + inp + "\"")
		layer.AddText("How about start?")
		layer.RegisterIButton("Start", s.start())

		err := s.botFrame.SendMsg(event.ChatID, layer)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		return nil
	}
}

func (s Service) processName() bf.HandlerFunc {
	return func(ctx context.Context, event bf.Event) error {
		name, err := getStringFromButtonOrPrompt(event)
		if err != nil {
			return fmt.Errorf("can't get name: %w", err)
		}

		if name == "SomeBadGuy" {
			return s.processBannedName()(ctx, event)
		}

		layer := s.botFrame.NewLayer()
		layer.AddText("Hello, " + name)
		layer.AddText("How are you? From 1 to 5")
		layer.RegisterIButton("1", s.processWorstFeeling(name, "1"))
		layer.RegisterIButton("2", s.processWorstFeeling(name, "2"))
		layer.RegisterIButton("3", s.processNeutralFeeling(name, "3"))
		layer.RegisterIButton("4", s.processNeutralFeeling(name, "4"))
		layer.RegisterIButton("5", s.processGoodFeeling(name, "5"))

		err = s.botFrame.SendMsg(event.ChatID, layer)
		if err != nil {
			return fmt.Errorf("can't send message with greeting: %w", err)
		}

		return nil
	}
}

func (s Service) processBannedName() bf.HandlerFunc {
	return func(ctx context.Context, event bf.Event) error {
		name, err := getStringFromButtonOrPrompt(event)
		if err != nil {
			return fmt.Errorf("can't get name: %w", err)
		}

		err = s.botFrame.SendText(event.ChatID, "Sorry, but you are banned, "+name)
		if err != nil {
			return fmt.Errorf("can't send text: %w", err)
		}

		err = s.botFrame.RetryLastLayer(event, "Choose another name")
		if err != nil {
			return fmt.Errorf("can't retry last layer: %w", err)
		}

		return nil
	}
}

func (s Service) processWorstFeeling(name string, feelingScore string) bf.HandlerFunc {
	return func(ctx context.Context, event bf.Event) error {
		layer := s.botFrame.NewLayer()
		layer.AddText("Sorry, " + name + "!")
		layer.AddText("I hope you will feel better")
		layer.AddText("I will send you a joke")

		if feelingScore == "1" {
			layer.AddText("`" + s.repo.GetARandomGoodJoke() + "`")
		} else {
			layer.AddText("`" + s.repo.GetARandomJoke() + "`")
		}

		layer.AddText("Do you want to see more?")
		layer.RegisterIButton("Yes", s.processWorstFeeling(name, feelingScore))
		layer.RegisterIButton("No", s.start())

		err := s.botFrame.SendMsg(event.ChatID, layer)
		if err != nil {
			return fmt.Errorf("can't send message with joke: %w", err)
		}

		return nil
	}
}

func (s Service) processNeutralFeeling(name string, _ string) bf.HandlerFunc {
	return func(ctx context.Context, event bf.Event) error {
		layer := s.botFrame.NewLayer()
		layer.AddText("Ok, " + name + "!")
		layer.AddText("I hope you will feel better")
		layer.RegisterIButton("Back", s.start())

		err := s.botFrame.SendMsg(event.ChatID, layer)
		if err != nil {
			return fmt.Errorf("can't send message neutral feeling: %w", err)
		}

		return nil
	}
}

func (s Service) processGoodFeeling(name string, _ string) bf.HandlerFunc {
	return func(ctx context.Context, event bf.Event) error {
		layer := s.botFrame.NewLayer()
		layer.AddText("Great, " + name + "!")
		layer.AddText("I am glad to hear that")
		layer.AddText("If you have such a mood, you can type me a joke. Or just press \"Back\"")
		layer.RegisterIButton("How to write jokes?", s.help("how to write jokes?"))
		layer.RegisterText(bf.AnyText, s.processJoke())
		layer.RegisterIButton("Back", s.start())

		err := s.botFrame.SendMsg(event.ChatID, layer)
		if err != nil {
			return fmt.Errorf("can't send message: %w", err)
		}

		return nil
	}
}

func (s Service) processJoke() bf.HandlerFunc {
	return func(ctx context.Context, event bf.Event) error {
		joke := event.Text
		layer := s.botFrame.NewLayer()
		layer.AddText("Aha-ha! That is a good joke:")
		layer.AddText(joke)
		layer.AddText("Let me save it?")
		layer.RegisterIButton("Yes", s.saveJoke(joke))
		layer.RegisterIButton("No", s.start())

		err := s.botFrame.SendMsg(event.ChatID, layer)
		if err != nil {
			return fmt.Errorf("can't send message with process joke: %w", err)
		}

		return nil
	}
}

func (s Service) saveJoke(joke string) bf.HandlerFunc {
	return func(ctx context.Context, event bf.Event) error {
		layer := s.botFrame.NewLayer()
		layer.AddText("Ok, I will save it")
		s.repo.SaveJoke(joke)
		layer.AddText("Thx you!")
		layer.RegisterIButton("Back", s.start())

		err := s.botFrame.SendMsg(event.ChatID, layer)
		if err != nil {
			return fmt.Errorf("can't send message with save joke: %w", err)
		}

		return nil
	}
}

func getStringFromButtonOrPrompt(event bf.Event) (string, error) {
	switch event.Kind {
	case bf.EventKindText:
		return event.Text, nil
	case bf.EventKindInlineButton:
		return event.ButtonText, nil
	case bf.EventKindCommand, bf.EventKindVoice:
		fallthrough
	default:
		return "", errors.New("unexpected event kind")
	}
}
