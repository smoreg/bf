package bf

import (
	"sort"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

// HandlerLayer groups handlers that may fire for events from one chat.
//
// Two common use cases:
//  1. Build a layer and pass it to SendMsg — the layer matches the very next
//     message from the user, then is wiped.
//  2. The bot's default layer, used whenever no chat-specific layer is set.
//     The default layer is never wiped automatically.
type HandlerLayer struct {
	text string

	commandHandler map[string]CommandHandler
	textHandler    map[string]TextHandler
	buttonHandler  map[string]InlineButtonHandler
	audioHandler   *AudioHandler

	layerDefaultHandler HandlerFunc

	ttl                time.Time
	generalMiddlewares []MiddlewareFunc
	rowMode            bool
}

// Handler returns the HandlerFunc that should process the given event,
// or the layer's default handler if no specific handler is registered.
func (hl *HandlerLayer) Handler(event Event) HandlerFunc {
	switch event.Kind {
	case EventKindText:
		if h, ok := hl.textHandler[event.Text]; ok {
			return h.handlerFunc
		}
		if h, ok := hl.textHandler[AnyText]; ok {
			return h.handlerFunc
		}
	case EventKindCommand:
		if h, ok := hl.commandHandler["/"+event.Command]; ok {
			return h.handlerFunc
		}
	case EventKindInlineButton:
		if h, ok := hl.buttonHandler[event.Button]; ok {
			return h.handlerFunc
		}
	case EventKindVoice:
		if hl.audioHandler != nil {
			return hl.audioHandler.handlerFunc
		}
	}

	return hl.layerDefaultHandler
}

// IsExpired reports whether the layer's TTL has elapsed.
func (hl *HandlerLayer) IsExpired() bool {
	return time.Now().After(hl.ttl)
}

// IsEmpty reports whether the layer has no registered handlers at all.
func (hl *HandlerLayer) IsEmpty() bool {
	return len(hl.textHandler) == 0 &&
		len(hl.commandHandler) == 0 &&
		len(hl.buttonHandler) == 0 &&
		hl.layerDefaultHandler == nil
}

// InlineButtonHandler is one inline-keyboard button bound to a handler.
type InlineButtonHandler struct {
	text        string
	id          uuid.UUID
	handlerFunc HandlerFunc
	orderWeight int
	button      tgbotapi.InlineKeyboardButton
}

// TextHandlerKind discriminates plain text matches from reply-keyboard buttons.
type TextHandlerKind string

// Text-handler kinds and the wildcard text token.
const (
	TextHandlerKindText   TextHandlerKind = "text"
	TextHandlerKindButton TextHandlerKind = "button"
	// AnyText is the wildcard text-handler key: when registered, it matches
	// any incoming text message that has no exact handler.
	AnyText = "*"
)

// TextHandler matches an incoming text message (literal or via AnyText).
type TextHandler struct {
	text        string
	id          uuid.UUID
	handlerFunc HandlerFunc
	kind        TextHandlerKind
	orderWeight int
}

// AudioHandler matches voice messages.
type AudioHandler struct {
	id          uuid.UUID
	handlerFunc HandlerFunc
}

// CommandHandler matches a slash command (e.g. "/start").
type CommandHandler struct {
	command     string
	id          uuid.UUID
	handlerFunc HandlerFunc
}

// RegisterCommand binds a handler to a slash command (must include the slash).
func (hl *HandlerLayer) RegisterCommand(command string, handler HandlerFunc) {
	hl.commandHandler[command] = CommandHandler{
		command:     command,
		id:          uuid.New(),
		handlerFunc: handler,
	}
}

// RegisterText binds a handler to an exact text message.
// Pass AnyText as text to match any text message.
func (hl *HandlerLayer) RegisterText(text string, handler HandlerFunc) {
	hl.textHandler[text] = TextHandler{
		text:        text,
		id:          uuid.New(),
		handlerFunc: handler,
		kind:        TextHandlerKindText,
	}
}

// RegisterButton adds a reply-keyboard button. The button text doubles as the match key.
func (hl *HandlerLayer) RegisterButton(text string, handler HandlerFunc) {
	hl.textHandler[text] = TextHandler{
		text:        text,
		id:          uuid.New(),
		handlerFunc: handler,
		kind:        TextHandlerKindButton,
		orderWeight: len(hl.textHandler),
	}
}

// RegisterIButton adds an inline-keyboard button with a callback handler.
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

// RegisterIButtonURL adds an inline-keyboard button that opens a URL.
// No handler is invoked when the user taps it.
func (hl *HandlerLayer) RegisterIButtonURL(text, url string) {
	id := uuid.New()

	hl.buttonHandler[id.String()] = InlineButtonHandler{
		button:      tgbotapi.NewInlineKeyboardButtonURL(text, url),
		text:        text,
		id:          id,
		handlerFunc: nil,
		orderWeight: len(hl.buttonHandler),
	}
}

// RegisterIButtonSwitch adds an inline-keyboard "switch inline query" button.
func (hl *HandlerLayer) RegisterIButtonSwitch(text, link string, handler HandlerFunc) {
	id := uuid.New()
	hl.buttonHandler[id.String()] = InlineButtonHandler{
		button:      tgbotapi.NewInlineKeyboardButtonSwitch(text, link),
		text:        text,
		id:          id,
		handlerFunc: handler,
		orderWeight: len(hl.buttonHandler),
	}
}

// AddText appends a line to the layer's message text.
// The first call sets the text directly; subsequent calls join with a newline.
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

func (hl *HandlerLayer) sortedButtonsSlice() []TextHandler {
	res := make([]TextHandler, 0, len(hl.textHandler))
	for _, v := range hl.textHandler {
		if v.kind == TextHandlerKindButton {
			res = append(res, v)
		}
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].orderWeight < res[j].orderWeight
	})

	return res
}

// SetIButtonRowMode lays the inline buttons in a single row, with the last
// button moved to a row of its own. Useful for an "OK / Cancel" footer.
func (hl *HandlerLayer) SetIButtonRowMode() {
	hl.rowMode = true
}

// RegisterVoice binds a handler to incoming voice messages on this layer.
func (hl *HandlerLayer) RegisterVoice(handler HandlerFunc) {
	hl.audioHandler = &AudioHandler{
		id:          uuid.New(),
		handlerFunc: handler,
	}
}
