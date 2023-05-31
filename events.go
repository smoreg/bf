package bf

import (
	"encoding/json"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/pkg/errors"
)

// Event is a struct that represents event from telegram.
type Event struct {
	Kind             eventType `json:"kind"`
	Text             string    `json:"text"`
	Command          string    `json:"command"`
	Button           string    `json:"button"`
	ButtonText       string    `json:"buttonText"`
	ChatID           int64     `json:"chatId"`
	UserTGID         int64     `json:"userTgId"`
	FirstName        string    `json:"firstName"`
	LastName         string    `json:"lastName"`
	UserTgUsername   string    `json:"userTgUsername"`
	CommandArguments string    `json:"commandArguments"`
	Username         string    `json:"username"`
	lastLayer        *HandlerLayer
	Voice            *tgbotapi.Voice `json:"-"`
}

func (e *Event) String() string {
	return fmt.Sprintf("%#v\n", e)
}

func (e *Event) json() (string, error) {
	ind, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal event to json")
	}

	return string(ind), nil
}

func (e *Event) FullName() string {
	return e.FirstName + " " + e.LastName
}

// newEvent creates new event from tg api update object.
func newEvent(update tgbotapi.Update) (Event, bool) {
	event := Event{}

	var from *tgbotapi.User

	switch {
	case update.Message != nil && update.Message.Voice != nil:
		event.Kind = EventKindVoice
		event.Voice = update.Message.Voice
		event.ChatID = update.Message.Chat.ID
		from = update.Message.From
	case update.Message != nil && update.Message.IsCommand():
		event.Kind = EventKindCommand
		event.Command = update.Message.Command()
		event.ChatID = update.Message.Chat.ID
		event.CommandArguments = update.Message.CommandArguments()
		from = update.Message.From
	case update.Message != nil:
		event.Kind = EventKindText
		event.Text = update.Message.Text
		event.ChatID = update.Message.Chat.ID
		from = update.Message.From
	case update.CallbackQuery != nil:
		event.Kind = EventKindInlineButton
		event.Button = update.CallbackQuery.Data
	L:
		for _, row := range update.CallbackQuery.Message.ReplyMarkup.InlineKeyboard {
			for _, button := range row {
				if button.CallbackData == nil {
					continue
				}
				data := *button.CallbackData
				if data == update.CallbackQuery.Data {
					event.ButtonText = button.Text

					break L
				}
			}
		}

		event.ChatID = update.CallbackQuery.Message.Chat.ID
		from = update.CallbackQuery.From

	default:
		return event, false
	}

	event.UserTGID = from.ID
	event.FirstName = from.FirstName
	event.LastName = from.LastName
	event.Username = from.UserName
	event.UserTgUsername = from.UserName

	return event, true
}
