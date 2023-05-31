package bf

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (b *ChatBotImpl) defaultErrorHandler(_ context.Context, event Event, err error) {
	logrus.WithError(err).Errorf("failed to process event: %+v", event)
}

func (b *ChatBotImpl) defaultEventHandler(_ context.Context, event Event) error {
	if b.debug {
		jsonView, err := event.json()
		if err != nil {
			return errors.Wrap(err, "failed to marshal event to json")
		}

		return b.SendText(event.ChatID, "I don't know what to do with this event: \n"+jsonView+"\n")
	}

	return nil
}
