package agent

import (
	"sync"
	"time"
)

type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityUrgent
)

type BufferedEvent struct {
	Name      string
	Payload   any
	Priority  Priority
	TickID    uint64
	Timestamp time.Time
}

type EventBuffer struct {
	mu       sync.Mutex
	events   []BufferedEvent
	capacity int
}

func NewEventBuffer(capacity int) *EventBuffer {
	if capacity <= 0 {
		capacity = 100
	}
	return &EventBuffer{capacity: capacity}
}

func (b *EventBuffer) Push(name string, payload any, priority Priority) {
	b.PushAt(name, payload, priority, 0)
}

func (b *EventBuffer) PushAt(name string, payload any, priority Priority, tickID uint64) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	evt := BufferedEvent{
		Name:      name,
		Payload:   payload,
		Priority:  priority,
		TickID:    tickID,
		Timestamp: time.Now(),
	}

	if len(b.events) < b.capacity {
		b.events = append(b.events, evt)
		return
	}

	idx := b.findDropIndexLocked()
	if idx >= 0 {
		b.events = append(b.events[:idx], b.events[idx+1:]...)
		b.events = append(b.events, evt)
		return
	}

	if priority == PriorityUrgent {
		b.events = append(b.events[1:], evt)
	}
}

func (b *EventBuffer) DrainAll() []BufferedEvent {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]BufferedEvent, len(b.events))
	copy(out, b.events)
	b.events = b.events[:0]
	return out
}

func (b *EventBuffer) HasUrgent() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, evt := range b.events {
		if evt.Priority == PriorityUrgent {
			return true
		}
	}
	return false
}

func (b *EventBuffer) Len() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

func (b *EventBuffer) findDropIndexLocked() int {
	for i, evt := range b.events {
		if evt.Priority == PriorityLow {
			return i
		}
	}
	for i, evt := range b.events {
		if evt.Priority == PriorityNormal {
			return i
		}
	}
	return -1
}
