package agent

import "testing"

func TestEventBufferPushDrainAndUrgent(t *testing.T) {
	b := NewEventBuffer(4)
	b.Push("entity.appear", 1, PriorityLow)
	b.Push("chat", 2, PriorityUrgent)

	if !b.HasUrgent() {
		t.Fatal("expected urgent event")
	}
	if b.Len() != 2 {
		t.Fatalf("len=%d want 2", b.Len())
	}

	drained := b.DrainAll()
	if len(drained) != 2 {
		t.Fatalf("drained len=%d want 2", len(drained))
	}
	if drained[0].TickID != 0 || drained[1].TickID != 0 {
		t.Fatalf("default push should keep tickID=0, got %d and %d", drained[0].TickID, drained[1].TickID)
	}
	if b.Len() != 0 {
		t.Fatalf("len after drain=%d want 0", b.Len())
	}
}

func TestEventBufferOverflowDropsLowFirst(t *testing.T) {
	b := NewEventBuffer(3)
	b.Push("low1", nil, PriorityLow)
	b.Push("normal1", nil, PriorityNormal)
	b.Push("urgent1", nil, PriorityUrgent)
	b.Push("normal2", nil, PriorityNormal)

	drained := b.DrainAll()
	if len(drained) != 3 {
		t.Fatalf("drained len=%d want 3", len(drained))
	}

	names := map[string]struct{}{}
	for _, evt := range drained {
		names[evt.Name] = struct{}{}
	}
	if _, ok := names["low1"]; ok {
		t.Fatal("expected low1 to be dropped on overflow")
	}
}

func TestEventBufferPushAtCarriesTickID(t *testing.T) {
	b := NewEventBuffer(2)
	b.PushAt("damage", nil, PriorityUrgent, 42)
	drained := b.DrainAll()
	if len(drained) != 1 {
		t.Fatalf("drained len=%d want 1", len(drained))
	}
	if drained[0].TickID != 42 {
		t.Fatalf("tickID=%d want 42", drained[0].TickID)
	}
}
