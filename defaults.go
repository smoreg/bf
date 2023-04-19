package bf

import (
	"context"

	"github.com/sirupsen/logrus"
)

func (b *botBuilder) defaultErrorHandler(ctx context.Context, event Event, err error) {
	logrus.WithError(err).Errorf("failed to process event: %+v", event)
}

func (b *botBuilder) defaultEventHandler(ctx context.Context, event Event) error {
	if b.debug {
		return b.SendText(event.ChatID, "I don't know what to do with this event: \n"+event.json()+"\n")
	}
	return nil
}
