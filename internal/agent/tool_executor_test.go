package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/world"
)

func TestToolExecutorLook(t *testing.T) {
	blocks := newCameraTestBlocks()
	blocks.set(0, 2, 3, 1)

	executor := ToolExecutor{
		SnapshotFn: func() world.Snapshot {
			return world.Snapshot{Position: world.Position{X: 0.5, Y: 1, Z: 0.5, Yaw: 0, Pitch: 0}}
		},
		World:  blocks,
		Camera: Camera{FOV: 70, MaxDist: 16, Width: 1, Height: 1},
	}

	text, err := executor.ExecuteTool(context.Background(), "look", map[string]any{"direction": "forward"})
	if err != nil {
		t.Fatalf("ExecuteTool look error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("parse look result json: %v", err)
	}
	blocksText, _ := out["blocks"].(string)
	if !strings.Contains(strings.ToLower(blocksText), "stone") {
		t.Fatalf("expected stone in look blocks, got %q", blocksText)
	}
}

func TestToolExecutorSpeakAndSetIntent(t *testing.T) {
	speakCh := make(chan string, 1)
	intentCh := make(chan Intent, 1)

	executor := ToolExecutor{
		SpeakChan:  speakCh,
		IntentChan: intentCh,
	}

	if _, err := executor.ExecuteTool(context.Background(), "speak", map[string]any{"message": "hello"}); err != nil {
		t.Fatalf("speak error: %v", err)
	}
	select {
	case got := <-speakCh:
		if got != "hello" {
			t.Fatalf("speak payload=%q want hello", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting speak channel")
	}

	if _, err := executor.ExecuteTool(context.Background(), "set_intent", map[string]any{"action": "go_to", "x": 1, "y": 2, "z": 3}); err != nil {
		t.Fatalf("set_intent error: %v", err)
	}
	select {
	case got := <-intentCh:
		if got.Action != "go_to" {
			t.Fatalf("intent action=%q want go_to", got.Action)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting intent channel")
	}
}

func TestToolExecutorCheckInventoryUnavailable(t *testing.T) {
	executor := ToolExecutor{}
	text, err := executor.ExecuteTool(context.Background(), "check_inventory", nil)
	if err != nil {
		t.Fatalf("check_inventory error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("parse result json: %v", err)
	}
	if out["status"] != "unavailable" {
		t.Fatalf("status=%v want unavailable", out["status"])
	}
	if out["reason"] != "inventory_not_ready" {
		t.Fatalf("reason=%v want inventory_not_ready", out["reason"])
	}
}

func TestToolExecutorWaitForIdle(t *testing.T) {
	executor := ToolExecutor{
		WaitForIdle: func(ctx context.Context, timeout time.Duration) (map[string]any, error) {
			if timeout != 5*time.Second {
				t.Fatalf("timeout=%v want 5s", timeout)
			}
			return map[string]any{"status": "idle"}, nil
		},
	}

	text, err := executor.ExecuteTool(context.Background(), "wait_for_idle", map[string]any{"timeout_ms": 5000})
	if err != nil {
		t.Fatalf("wait_for_idle error: %v", err)
	}
	if !strings.Contains(text, "idle") {
		t.Fatalf("result=%q should contain idle", text)
	}
}

func TestToolExecutorLookUpdatesSpatialMemory(t *testing.T) {
	blocks := newCameraTestBlocks()
	blocks.set(0, 2, 3, 1)
	memory := NewSpatialMemory()

	executor := ToolExecutor{
		SnapshotFn: func() world.Snapshot {
			return world.Snapshot{Position: world.Position{X: 0.5, Y: 1, Z: 0.5, Yaw: 0, Pitch: 0}, Entities: []world.Entity{{EntityID: 42, Type: 150, X: 0.5, Y: 1, Z: 3.5}}}
		},
		World:         blocks,
		Camera:        Camera{FOV: 70, MaxDist: 16, Width: 1, Height: 1},
		SpatialMemory: memory,
		TickIDFn:      func() uint64 { return 123 },
	}

	if _, err := executor.ExecuteTool(context.Background(), "look", map[string]any{"direction": "forward"}); err != nil {
		t.Fatalf("ExecuteTool look error: %v", err)
	}

	entities, rememberedBlocks := memory.QueryNearby(Vec3{X: 0.5, Y: 1, Z: 0.5}, 16, time.Minute)
	if len(entities) == 0 {
		t.Fatal("expected look to update entity memory")
	}
	if len(rememberedBlocks) == 0 {
		t.Fatal("expected look to update block memory")
	}
}

func TestToolExecutorQueryNearby(t *testing.T) {
	memory := NewSpatialMemory()
	memory.UpdateEntities([]world.Entity{{EntityID: 9, Type: 150, X: 2, Y: 64, Z: 1}}, 7)
	memory.UpdateBlocks([]BlockInfo{{Type: "oak_log", Pos: [3]int{1, 64, 1}}}, 7)

	executor := ToolExecutor{
		SnapshotFn: func() world.Snapshot {
			return world.Snapshot{Position: world.Position{X: 0, Y: 64, Z: 0}}
		},
		SpatialMemory: memory,
	}

	text, err := executor.ExecuteTool(context.Background(), "query_nearby", map[string]any{"radius": 16, "type_filter": "all", "max_age_sec": 30})
	if err != nil {
		t.Fatalf("query_nearby error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("parse query_nearby result json: %v", err)
	}
	if out["status"] != "ok" {
		t.Fatalf("status=%v want ok", out["status"])
	}
	summary, _ := out["summary"].(string)
	if !strings.Contains(summary, "id=9") || !strings.Contains(summary, "oak_log") {
		t.Fatalf("summary=%q should include remembered entity and block", summary)
	}
}

func TestToolExecutorQueryNearbySummaryMatchesFilterAndAge(t *testing.T) {
	memory := NewSpatialMemory()
	memory.UpdateEntities([]world.Entity{{EntityID: 11, Type: 150, X: 2, Y: 64, Z: 1}}, 8)
	pos := [3]int{1, 64, 1}
	memory.UpdateBlocks([]BlockInfo{{Type: "oak_log", Pos: pos}}, 8)

	memory.mu.Lock()
	block := memory.blocks[pos]
	block.LastSeen = time.Now().Add(-5 * time.Second)
	memory.blocks[pos] = block
	memory.mu.Unlock()

	executor := ToolExecutor{
		SnapshotFn: func() world.Snapshot {
			return world.Snapshot{Position: world.Position{X: 0, Y: 64, Z: 0}}
		},
		SpatialMemory: memory,
	}

	text, err := executor.ExecuteTool(context.Background(), "query_nearby", map[string]any{"radius": 16, "type_filter": "entity", "max_age_sec": 1})
	if err != nil {
		t.Fatalf("query_nearby error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("parse query_nearby result json: %v", err)
	}
	blocksAny, _ := out["blocks"].([]any)
	if len(blocksAny) != 0 {
		t.Fatalf("blocks len=%d want 0 for entity filter", len(blocksAny))
	}
	entitiesAny, _ := out["entities"].([]any)
	if len(entitiesAny) == 0 {
		t.Fatal("expected at least one entity in query result")
	}
	summary, _ := out["summary"].(string)
	if strings.Contains(summary, "oak_log") {
		t.Fatalf("summary=%q should not contain filtered/expired block", summary)
	}
	if !strings.Contains(summary, "Nearby entities (last 1s)") {
		t.Fatalf("summary=%q should use max_age_sec window", summary)
	}
	if !strings.Contains(summary, "Recent blocks: none") {
		t.Fatalf("summary=%q should match filtered empty block list", summary)
	}
}
