package event

import (
	"log/slog"
	"sync"
)

type HandlerFunc func(raw any)

type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]HandlerFunc
}

func NewBus() *Bus {
	return &Bus{
		handlers: make(map[string][]HandlerFunc),
	}
}

func (b *Bus) Subscribe(eventName string, handler HandlerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventName] = append(b.handlers[eventName], handler)
}

func (b *Bus) Publish(eventName string, evt any) {
	b.mu.RLock()
	handlers := make([]HandlerFunc, len(b.handlers[eventName]))
	copy(handlers, b.handlers[eventName])
	b.mu.RUnlock()

	for _, handler := range handlers {
		go func(h HandlerFunc) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Event handler panicked", "event", eventName, "panic", r)
				}
			}()
			h(evt)
		}(handler)
	}
}
