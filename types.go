package bf

import (
	"encoding/json"
	"fmt"
)

type eventType string

const (
	EventKindText         eventType = "text"
	EventKindInlineButton eventType = "buttonInline"
	EventKindCommand      eventType = "command"
)

type Event struct {
	Kind           eventType
	Text           string
	Command        string
	Button         string
	ButtonText     string
	ChatID         int64
	UserTGID       int64
	FirstName      string
	LastName       string
	UserTgUsername string
	// todo fill parse queryString params on start like https://t.me/mybot?start=c5628dfe-e3c1-4ef7-891b-ba2307c257b7
	CommandArguments string
	Username         string
	lastLayer        *HandlerLayer
}

func (e *Event) String() string {
	return fmt.Sprintf("%#v\n", e)
}

func (e *Event) json() string {
	ind, _ := json.MarshalIndent(e, "", "  ")
	return string(ind)
}

func (e *Event) FullName() string {
	return e.FirstName + " " + e.LastName
}
