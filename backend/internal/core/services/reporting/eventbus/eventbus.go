// Package eventbus provides an interface for event bus.
package eventbus

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Event string

const (
	EventTagMutation    Event = "tags.mutation"
	EventEntityMutation Event = "entity.mutation"
	EventUserMutation   Event = "user.mutation"
	EventExportMutation Event = "export.mutation"
	EventImportMutation Event = "import.mutation"
)

type GroupMutationEvent struct {
	GID uuid.UUID
}

type eventData struct {
	event Event
	data  any
}

type EventBus struct {
	started atomic.Bool
	ch      chan eventData

	mu          sync.RWMutex
	subscribers map[Event][]func(any)
}

func New() *EventBus {
	return &EventBus{
		ch: make(chan eventData, 100),
		subscribers: map[Event][]func(any){
			EventTagMutation:    {},
			EventEntityMutation: {},
			EventUserMutation:   {},
			EventExportMutation: {},
			EventImportMutation: {},
		},
	}
}

func (e *EventBus) Run(ctx context.Context) error {
	if !e.started.CompareAndSwap(false, true) {
		panic("event bus already started")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-e.ch:
			e.mu.RLock()
			arr, ok := e.subscribers[event.event]
			e.mu.RUnlock()

			if !ok {
				continue
			}

			for _, fn := range arr {
				runSubscriber(fn, event.data)
			}
		}
	}
}

func runSubscriber(fn func(any), data any) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error().Interface("panic", recovered).Msg("event bus subscriber panicked")
		}
	}()

	fn(data)
}

func (e *EventBus) Publish(event Event, data any) {
	e.ch <- eventData{
		event: event,
		data:  data,
	}
}

func (e *EventBus) Subscribe(event Event, fn func(any)) {
	e.mu.Lock()
	defer e.mu.Unlock()

	arr, ok := e.subscribers[event]
	if !ok {
		panic("event not found")
	}

	e.subscribers[event] = append(arr, fn)
}
