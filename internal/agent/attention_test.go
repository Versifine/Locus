package agent

import (
	"testing"
	"time"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/world"
)

func TestAttentionPublishesDamage(t *testing.T) {
	bus := event.NewBus()
	attention := NewAttention(bus)

	ch := make(chan event.DamageEvent, 1)
	bus.Subscribe(event.EventDamage, func(raw any) {
		evt, ok := raw.(event.DamageEvent)
		if ok {
			ch <- evt
		}
	})

	attention.Tick(world.Snapshot{Health: 20})
	attention.Tick(world.Snapshot{Health: 17.5})

	select {
	case evt := <-ch:
		if evt.Amount != 2.5 {
			t.Fatalf("amount=%v want 2.5", evt.Amount)
		}
		if evt.NewHP != 17.5 {
			t.Fatalf("newhp=%v want 17.5", evt.NewHP)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting damage event")
	}
}

func TestAttentionPublishesEntityAppearLeave(t *testing.T) {
	bus := event.NewBus()
	attention := NewAttention(bus)

	appearCh := make(chan event.EntityEvent, 1)
	leaveCh := make(chan event.EntityEvent, 1)

	bus.Subscribe(event.EventEntityAppear, func(raw any) {
		evt, ok := raw.(event.EntityEvent)
		if ok {
			appearCh <- evt
		}
	})
	bus.Subscribe(event.EventEntityLeave, func(raw any) {
		evt, ok := raw.(event.EntityEvent)
		if ok {
			leaveCh <- evt
		}
	})

	attention.Tick(world.Snapshot{})
	attention.Tick(world.Snapshot{Entities: []world.Entity{{EntityID: 42, Type: 155}}})

	select {
	case evt := <-appearCh:
		if evt.EntityID != 42 {
			t.Fatalf("appear id=%d want 42", evt.EntityID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting appear event")
	}

	attention.Tick(world.Snapshot{})

	select {
	case evt := <-leaveCh:
		if evt.EntityID != 42 {
			t.Fatalf("leave id=%d want 42", evt.EntityID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting leave event")
	}
}
