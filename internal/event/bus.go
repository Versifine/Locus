package event

import "sync"

type HandlerFunc func(event any)

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

func (b *Bus) Publish(eventName string, event any) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if handlers, found := b.handlers[eventName]; found {
		for _, handler := range handlers {
			handler(event)
		}
	}
}
