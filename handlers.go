package bf

import (
	"sort"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

// HandlerLayer handlers for events from 1 chat.
// There is 2 most common usages:
// 1. Create layer and in use in SendMsg. That way your layer will be used for next 1 user message and wipe after.
// 2. Default layer. That layer process all messages that doesn't have any chat-specific one-time layers.
type HandlerLayer struct {
	text string

	commandHandler map[string]CommandHandler
	textHandler    map[string]TextHandler
	buttonHandler  map[string]InlineButtonHandler

	layerDefaultHandler HandlerFunc

	ttl                time.Time
	generalMiddlewares []MiddlewareFunc
}

// Handler get a HandlerFunc for event.
func (hl *HandlerLayer) Handler(event Event) HandlerFunc {
	switch event.Kind {
	case EventKindText:
		handlerText, ok := hl.textHandler[event.Text]
		if ok {
			return handlerText.handlerFunc
		}
		handlerText, ok = hl.textHandler["*"]
		if ok {
			return handlerText.handlerFunc
		}
	case EventKindCommand:
		handlerCommand, ok := hl.commandHandler["/"+event.Command]
		if ok {
			return handlerCommand.handlerFunc
		}
	case EventKindInlineButton:
		handlerButton, ok := hl.buttonHandler[event.Button]
		if ok {
			return handlerButton.handlerFunc
		}
	}
	return hl.layerDefaultHandler
}

func (hl *HandlerLayer) IsExpired() bool {
	return time.Now().After(hl.ttl)
}

func (hl *HandlerLayer) IsEmpty() bool {
	return len(hl.textHandler) == 0 &&
		len(hl.commandHandler) == 0 &&
		len(hl.buttonHandler) == 0 &&
		hl.layerDefaultHandler == nil
}

type InlineButtonHandler struct {
	text        string
	id          uuid.UUID
	handlerFunc HandlerFunc
	orderWeight int
	button      tgbotapi.InlineKeyboardButton // TODO data duplicate
}

type TextHandlerKind string

const (
	TextHandlerKindText   TextHandlerKind = "text"
	TextHandlerKindButton TextHandlerKind = "button"
	AnyText                               = "*"
)

type TextHandler struct {
	text        string
	id          uuid.UUID
	handlerFunc HandlerFunc
	kind        TextHandlerKind
}

type CommandHandler struct {
	command     string
	id          uuid.UUID
	handlerFunc HandlerFunc
}

func (hl *HandlerLayer) RegisterCommand(command string, handler HandlerFunc) {
	hl.commandHandler[command] = CommandHandler{
		command:     command,
		id:          uuid.New(),
		handlerFunc: handler,
	}
}

func (hl *HandlerLayer) RegisterText(text string, handler HandlerFunc) {

	hl.textHandler[text] = TextHandler{
		text:        text,
		id:          uuid.New(),
		handlerFunc: handler,
		kind:        TextHandlerKindText,
	}
}

func (hl *HandlerLayer) RegisterButton(text string, handler HandlerFunc) {
	hl.textHandler[text] = TextHandler{
		text:        text,
		id:          uuid.New(),
		handlerFunc: handler,
		kind:        TextHandlerKindButton,
	}
}

func (hl *HandlerLayer) RegisterIButton(text string, handler HandlerFunc) {
	id := uuid.New()

	hl.buttonHandler[id.String()] = InlineButtonHandler{
		button:      tgbotapi.NewInlineKeyboardButtonData(text, id.String()),
		text:        text,
		id:          id,
		handlerFunc: handler,
		orderWeight: len(hl.buttonHandler),
	}
}

func (hl *HandlerLayer) RegisterIButtonSwitch(text string, link string, handler HandlerFunc) {
	id := uuid.New()
	hl.buttonHandler[id.String()] = InlineButtonHandler{
		button:      tgbotapi.NewInlineKeyboardButtonSwitch(text, link),
		text:        text,
		id:          id,
		handlerFunc: handler,
		orderWeight: len(hl.buttonHandler),
	}
}

func (hl *HandlerLayer) AddText(text string) {
	if hl.text == "" {
		hl.text = text
		return
	}
	hl.text += "\n" + text
}

func (hl *HandlerLayer) sortedIButtonsSlice() []InlineButtonHandler {
	res := make([]InlineButtonHandler, 0, len(hl.buttonHandler))
	for _, v := range hl.buttonHandler {
		res = append(res, v)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].orderWeight < res[j].orderWeight
	})
	return res
}
