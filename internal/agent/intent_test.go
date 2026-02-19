package agent

import "testing"

func TestParseIntentGoTo(t *testing.T) {
	intent, err := ParseIntent(map[string]any{
		"action": "go_to",
		"x":      1,
		"y":      64,
		"z":      2,
		"sprint": true,
	})
	if err != nil {
		t.Fatalf("ParseIntent error: %v", err)
	}
	if intent.Action != "go_to" {
		t.Fatalf("action=%q want go_to", intent.Action)
	}
	if intent.Params["x"] != 1 || intent.Params["y"] != 64 || intent.Params["z"] != 2 {
		t.Fatalf("unexpected params: %+v", intent.Params)
	}
	if intent.Params["sprint"] != true {
		t.Fatalf("sprint=%v want true", intent.Params["sprint"])
	}
}

func TestParseIntentFollowSprint(t *testing.T) {
	intent, err := ParseIntent(map[string]any{
		"action":    "follow",
		"entity_id": 42,
		"distance":  2.5,
		"sprint":    true,
	})
	if err != nil {
		t.Fatalf("ParseIntent error: %v", err)
	}
	if intent.Action != "follow" {
		t.Fatalf("action=%q want follow", intent.Action)
	}
	if intent.Params["entity_id"] != 42 {
		t.Fatalf("entity_id=%v want 42", intent.Params["entity_id"])
	}
	if intent.Params["distance"] != 2.5 {
		t.Fatalf("distance=%v want 2.5", intent.Params["distance"])
	}
	if intent.Params["sprint"] != true {
		t.Fatalf("sprint=%v want true", intent.Params["sprint"])
	}
}

func TestParseIntentLookAtEntity(t *testing.T) {
	intent, err := ParseIntent(map[string]any{"action": "look_at", "entity_id": 42})
	if err != nil {
		t.Fatalf("ParseIntent error: %v", err)
	}
	if intent.Params["entity_id"] != 42 {
		t.Fatalf("entity_id=%v want 42", intent.Params["entity_id"])
	}
}

func TestParseIntentMissingField(t *testing.T) {
	_, err := ParseIntent(map[string]any{"action": "attack"})
	if err == nil {
		t.Fatal("expected error for missing entity_id")
	}
}

func TestParseIntentUnknownAction(t *testing.T) {
	_, err := ParseIntent(map[string]any{"action": "dance"})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestParseIntentSlotValidation(t *testing.T) {
	_, err := ParseIntent(map[string]any{"action": "switch_slot", "slot": 12})
	if err == nil {
		t.Fatal("expected slot range validation error")
	}
}
