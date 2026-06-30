package events

import (
	"context"
)

type Event interface {
	EventName() string
}

type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(event Event, handler Handler) error
}

type Handler func(context.Context, Event) error
