package events

import (
	"context"
	"errors"
	"reflect"
	"sync"
)

type inMemoryEventBus struct {
	handlers map[reflect.Type][]Handler
	mu       sync.RWMutex
}

func NewInMemoryEventBus() *inMemoryEventBus {
	return &inMemoryEventBus{
		handlers: make(map[reflect.Type][]Handler),
	}
}
func (b *inMemoryEventBus) Publish(ctx context.Context, event Event) error {
	if event == nil {
		return errors.New("event is nil")
	}
	evtType := reflect.TypeOf(event)

	b.mu.RLock()
	handlers, ok := b.handlers[evtType]
	if ok {
		// Copy handlers under lock to avoid holding the mutex during callback execution.
		handlers = append([]Handler(nil), handlers...)
	}
	b.mu.RUnlock()

	if !ok {
		return nil
	}
	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (b *inMemoryEventBus) Subscribe(event Event, handler Handler) error {
	if event == nil {
		return errors.New("event is nil")
	}
	if handler == nil {
		return errors.New("handler is nil")
	}
	evtType := reflect.TypeOf(event)

	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[evtType] = append(b.handlers[evtType], handler)
	return nil
}
