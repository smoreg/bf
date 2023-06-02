package bf

import (
	"context"

	"github.com/pkg/errors"
)

func (b *ChatBotImpl) defaultErrorHandler(_ context.Context, event Event, err error) {
	b.logger.Errorf("defaultErrorHandler process error: %+v for event: %+v", err, event)
}

func (b *ChatBotImpl) defaultEventHandler(_ context.Context, event Event) error {
	b.logger.Debugf("defaultEventHandler process event: %+v", event)

	if b.debug {
		jsonView, err := event.json()
		if err != nil {
			return errors.Wrap(err, "failed to marshal event to json")
		}

		return b.SendText(event.ChatID, "I don't know what to do with this event: \n"+jsonView+"\n")
	}

	return nil
}
