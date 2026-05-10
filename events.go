package bf

import (
	"encoding/json"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Event is a normalised representation of a Telegram update consumed by handlers.
type Event struct {
	Kind             EventKind `json:"kind"`
	Text             string    `json:"text"`
	Command          string    `json:"command"`
	Button           string    `json:"button"`
	ButtonText       string    `json:"buttonText"`
	ChatID           int64     `json:"chatID"`
	UserTGID         int64     `json:"userTGID"`
	FirstName        string    `json:"firstName"`
	LastName         string    `json:"lastName"`
	CommandArguments string    `json:"commandArguments"`
	Username         string    `json:"username"`
	lastLayer        *HandlerLayer
	Voice            *tgbotapi.Voice `json:"-"`
}

// String renders the event in Go syntax for debug logging.
// No trailing newline (unlike fmt.Sprintln) so the standard fmt.Stringer contract holds.
func (e *Event) String() string {
	return fmt.Sprintf("%#v", e)
}

func (e *Event) json() (string, error) {
	ind, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal event to json: %w", err)
	}

	return string(ind), nil
}

// FullName returns "FirstName LastName" with a single separating space.
func (e *Event) FullName() string {
	return e.FirstName + " " + e.LastName
}

// newEvent normalises a tgbotapi.Update into an Event.
// Returns ok=false if the update carries no payload the framework understands
// or if a required nested field (Message.Chat, CallbackQuery.Message.Chat) is
// missing — those updates cannot be replied to.
func newEvent(update tgbotapi.Update) (Event, bool) {
	event := Event{}

	var from *tgbotapi.User

	switch {
	case update.Message != nil && update.Message.Voice != nil:
		if update.Message.Chat == nil {
			return event, false
		}
		event.Kind = EventKindVoice
		event.Voice = update.Message.Voice
		event.ChatID = update.Message.Chat.ID
		from = update.Message.From
	case update.Message != nil && update.Message.IsCommand():
		if update.Message.Chat == nil {
			return event, false
		}
		event.Kind = EventKindCommand
		event.Command = update.Message.Command()
		event.ChatID = update.Message.Chat.ID
		event.CommandArguments = update.Message.CommandArguments()
		from = update.Message.From
	case update.Message != nil:
		if update.Message.Chat == nil {
			return event, false
		}
		event.Kind = EventKindText
		event.Text = update.Message.Text
		event.ChatID = update.Message.Chat.ID
		from = update.Message.From
	case update.CallbackQuery != nil:
		event.Kind = EventKindInlineButton
		event.Button = update.CallbackQuery.Data
		event.ButtonText = lookupCallbackButtonText(update.CallbackQuery)

		if update.CallbackQuery.Message != nil && update.CallbackQuery.Message.Chat != nil {
			event.ChatID = update.CallbackQuery.Message.Chat.ID
		}
		from = update.CallbackQuery.From

	default:
		return event, false
	}

	if from != nil {
		event.UserTGID = from.ID
		event.FirstName = from.FirstName
		event.LastName = from.LastName
		event.Username = from.UserName
	}

	return event, true
}

// lookupCallbackButtonText finds the button label that produced the callback
// by walking the inline-keyboard markup. Safe against nil Message / ReplyMarkup.
func lookupCallbackButtonText(q *tgbotapi.CallbackQuery) string {
	if q == nil || q.Message == nil || q.Message.ReplyMarkup == nil {
		return ""
	}

	for _, row := range q.Message.ReplyMarkup.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData == nil {
				continue
			}
			if *button.CallbackData == q.Data {
				return button.Text
			}
		}
	}
	return ""
}
