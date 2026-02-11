package world

import (
	"strings"
	"testing"
)

func TestSnapshotString_ItemEntityIncludesItemName(t *testing.T) {
	ws := &WorldState{}
	ws.AddEntity(Entity{
		EntityID: 1,
		Type:     71,
		X:        1,
		Y:        64,
		Z:        1,
		ItemName: "Egg",
	})

	snapshot := ws.GetState()
	got := snapshot.String()
	if !strings.Contains(got, "Item(Egg) ID:1") {
		t.Fatalf("Snapshot.String() = %q, want contains %q", got, "Item(Egg) ID:1")
	}
}

func TestUpdateEntityItemName_AppliesAfterSpawn(t *testing.T) {
	ws := &WorldState{}

	// Metadata can arrive before Spawn Entity.
	ws.UpdateEntityItemName(42, "Diamond")
	ws.AddEntity(Entity{
		EntityID: 42,
		Type:     71,
		X:        0,
		Y:        64,
		Z:        0,
	})

	snapshot := ws.GetState()
	if len(snapshot.Entities) != 1 {
		t.Fatalf("len(snapshot.Entities) = %d, want 1", len(snapshot.Entities))
	}
	if snapshot.Entities[0].ItemName != "Diamond" {
		t.Fatalf("snapshot.Entities[0].ItemName = %q, want %q", snapshot.Entities[0].ItemName, "Diamond")
	}
}
