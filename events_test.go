package bf

import (
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func msgUpdate(text string) tgbotapi.Update {
	return tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text: text,
			Chat: &tgbotapi.Chat{ID: 42},
			From: &tgbotapi.User{ID: 7, FirstName: "Ada", LastName: "Lovelace", UserName: "ada"},
		},
	}
}

func TestNewEvent_Text(t *testing.T) {
	ev, ok := newEvent(msgUpdate("hello"))
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.Kind != EventKindText || ev.Text != "hello" || ev.ChatID != 42 || ev.UserTGID != 7 {
		t.Fatalf("bad event: %+v", ev)
	}
	if ev.FullName() != "Ada Lovelace" {
		t.Fatalf("FullName: %q", ev.FullName())
	}
}

func TestNewEvent_Command(t *testing.T) {
	u := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text:     "/start arg1 arg2",
			Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
			Chat:     &tgbotapi.Chat{ID: 1},
			From:     &tgbotapi.User{ID: 1},
		},
	}
	ev, ok := newEvent(u)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.Kind != EventKindCommand || ev.Command != "start" || ev.CommandArguments != "arg1 arg2" {
		t.Fatalf("bad event: %+v", ev)
	}
}

func TestNewEvent_Voice(t *testing.T) {
	u := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Voice: &tgbotapi.Voice{FileID: "vfile"},
			Chat:  &tgbotapi.Chat{ID: 99},
			From:  &tgbotapi.User{ID: 5},
		},
	}
	ev, ok := newEvent(u)
	if !ok || ev.Kind != EventKindVoice || ev.Voice.FileID != "vfile" {
		t.Fatalf("bad event: ok=%v %+v", ok, ev)
	}
}

func TestNewEvent_CallbackQuery_FindsButtonText(t *testing.T) {
	data := "btn_id"
	u := tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{
			Data: data,
			From: &tgbotapi.User{ID: 1},
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 11},
				ReplyMarkup: &tgbotapi.InlineKeyboardMarkup{
					InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
						{{Text: "Click me", CallbackData: &data}},
					},
				},
			},
		},
	}
	ev, ok := newEvent(u)
	if !ok || ev.Kind != EventKindInlineButton || ev.Button != "btn_id" || ev.ButtonText != "Click me" || ev.ChatID != 11 {
		t.Fatalf("bad event: ok=%v %+v", ok, ev)
	}
}

func TestNewEvent_CallbackQuery_NilMessageSafe(t *testing.T) {
	u := tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{Data: "x", From: &tgbotapi.User{ID: 1}}}
	ev, ok := newEvent(u)
	if !ok {
		t.Fatal("expected ok=true even with nil message")
	}
	if ev.ButtonText != "" || ev.ChatID != 0 {
		t.Fatalf("expected zero ButtonText/ChatID, got %+v", ev)
	}
}

func TestNewEvent_CallbackQuery_NilReplyMarkupSafe(t *testing.T) {
	u := tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{
			Data:    "x",
			From:    &tgbotapi.User{ID: 1},
			Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 5}},
		},
	}
	ev, ok := newEvent(u)
	if !ok || ev.ChatID != 5 || ev.ButtonText != "" {
		t.Fatalf("bad event: ok=%v %+v", ok, ev)
	}
}

func TestNewEvent_Empty(t *testing.T) {
	_, ok := newEvent(tgbotapi.Update{})
	if ok {
		t.Fatal("expected ok=false for empty update")
	}
}

func TestEvent_StringAndJSON(t *testing.T) {
	ev := &Event{Kind: EventKindText, Text: "hi"}
	if !strings.Contains(ev.String(), `"hi"`) {
		t.Fatalf("String missing text: %q", ev.String())
	}
	j, err := ev.json()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(j, `"text": "hi"`) {
		t.Fatalf("json missing text: %q", j)
	}
}
