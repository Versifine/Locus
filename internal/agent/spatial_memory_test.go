package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/world"
)

func TestSpatialMemoryUpdateQueryMarkLeave(t *testing.T) {
	memory := NewSpatialMemory()
	memory.UpdateEntities([]world.Entity{{EntityID: 42, Type: 150, X: 10, Y: 64, Z: 20}}, 7)
	memory.UpdateBlocks([]BlockInfo{{Type: "oak_log", Pos: [3]int{11, 64, 20}}}, 7)

	entities, blocks := memory.QueryNearby(Vec3{X: 10, Y: 64, Z: 20}, 16, 30*time.Second)
	if len(entities) != 1 {
		t.Fatalf("entities len=%d want 1", len(entities))
	}
	if !entities[0].InFOV {
		t.Fatal("expected entity to be in_fov after update")
	}
	if len(blocks) != 1 || blocks[0].Name != "oak_log" {
		t.Fatalf("blocks=%v want oak_log x1", blocks)
	}

	memory.MarkEntityLeft(42, 8)
	entities, _ = memory.QueryNearby(Vec3{X: 10, Y: 64, Z: 20}, 16, 30*time.Second)
	if len(entities) != 1 {
		t.Fatalf("entities len=%d want 1 after leave", len(entities))
	}
	if entities[0].InFOV {
		t.Fatal("expected entity to be marked out of fov")
	}

	summary := memory.Summary(Vec3{X: 10, Y: 64, Z: 20}, 16)
	for _, want := range []string{"Nearby entities", "Recent blocks", "oak_log", "id=42"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary=%q missing %q", summary, want)
		}
	}
}

func TestSpatialMemoryGCRemovesExpiredEntitiesAndLimitsBlocks(t *testing.T) {
	memory := NewSpatialMemory()
	memory.maxEntityAge = 10 * time.Millisecond

	memory.UpdateEntities([]world.Entity{{EntityID: 1, Type: 150, X: 0, Y: 64, Z: 0}}, 1)
	time.Sleep(20 * time.Millisecond)

	for i := 0; i < maxSpatialMemoryBlocks+16; i++ {
		memory.UpdateBlocks([]BlockInfo{{Type: "stone", Pos: [3]int{i, 64, 0}}}, uint64(i+1))
	}

	memory.GC()

	entities, blocks := memory.QueryNearby(Vec3{X: 0, Y: 64, Z: 0}, 100000, time.Hour)
	if len(entities) != 0 {
		t.Fatalf("entities len=%d want 0 after GC", len(entities))
	}
	if len(blocks) != maxSpatialMemoryBlocks {
		t.Fatalf("blocks len=%d want %d", len(blocks), maxSpatialMemoryBlocks)
	}
}

func TestSpatialMemorySummaryNoData(t *testing.T) {
	memory := NewSpatialMemory()
	summary := memory.Summary(Vec3{X: 0, Y: 64, Z: 0}, 16)
	if !strings.Contains(summary, "Nearby entities (last 30s): none") {
		t.Fatalf("summary=%q should mention no entities", summary)
	}
	if !strings.Contains(summary, "Recent blocks: none") {
		t.Fatalf("summary=%q should mention no blocks", summary)
	}
}
