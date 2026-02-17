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
