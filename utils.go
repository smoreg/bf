package bf

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

func (b *botBuilder) ParseEvent(update tgbotapi.Update) (Event, bool) {
	event := Event{}
	var from *tgbotapi.User
	switch {
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
