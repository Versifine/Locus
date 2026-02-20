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

	attention.Tick(world.Snapshot{Health: 20}, 1)
	attention.Tick(world.Snapshot{Health: 17.5}, 2)

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

	attention.Tick(world.Snapshot{}, 1)
	attention.Tick(world.Snapshot{Entities: []world.Entity{{EntityID: 42, Type: 155}}}, 2)

	select {
	case evt := <-appearCh:
		if evt.EntityID != 42 {
			t.Fatalf("appear id=%d want 42", evt.EntityID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting appear event")
	}

	attention.Tick(world.Snapshot{}, 3)

	select {
	case evt := <-leaveCh:
		if evt.EntityID != 42 {
			t.Fatalf("leave id=%d want 42", evt.EntityID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting leave event")
	}
}

func TestAttentionUpdatesSpatialMemoryOnAppearAndLeave(t *testing.T) {
	attention := NewAttention(nil)
	attention.SpatialMemory = NewSpatialMemory()

	attention.Tick(world.Snapshot{}, 1)
	attention.Tick(world.Snapshot{Entities: []world.Entity{{EntityID: 9, Type: 150, X: 1, Y: 64, Z: 2}}}, 2)

	entities, _ := attention.SpatialMemory.QueryNearby(Vec3{X: 1, Y: 64, Z: 2}, 8, time.Minute)
	if len(entities) != 1 {
		t.Fatalf("entities len=%d want 1 after appear", len(entities))
	}
	if !entities[0].InFOV {
		t.Fatal("expected entity in_fov after appear")
	}

	attention.Tick(world.Snapshot{}, 3)
	entities, _ = attention.SpatialMemory.QueryNearby(Vec3{X: 1, Y: 64, Z: 2}, 8, time.Minute)
	if len(entities) != 1 {
		t.Fatalf("entities len=%d want 1 after leave", len(entities))
	}
	if entities[0].InFOV {
		t.Fatal("expected entity marked out_of_fov after leave")
	}
}

func TestAttentionRefreshesVisibleEntityAndPreventsGCExpiry(t *testing.T) {
	attention := NewAttention(nil)
	memory := NewSpatialMemory()
	memory.maxEntityAge = 25 * time.Millisecond
	attention.SpatialMemory = memory

	attention.Tick(world.Snapshot{Entities: []world.Entity{{EntityID: 7, Type: 150, X: 0, Y: 64, Z: 0}}}, 1)
	time.Sleep(20 * time.Millisecond)
	attention.Tick(world.Snapshot{Entities: []world.Entity{{EntityID: 7, Type: 150, X: 5, Y: 64, Z: 0}}}, 2)
	time.Sleep(20 * time.Millisecond)
	memory.GC()

	entities, _ := memory.QueryNearby(Vec3{X: 5, Y: 64, Z: 0}, 16, time.Minute)
	if len(entities) != 1 {
		t.Fatalf("entities len=%d want 1 after refresh+gc", len(entities))
	}
	if entities[0].X != 5 {
		t.Fatalf("entity X=%.1f want 5.0", entities[0].X)
	}
	if entities[0].TickID != 2 {
		t.Fatalf("entity tick=%d want 2", entities[0].TickID)
	}
}
