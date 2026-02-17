package skill

import "testing"

func TestMapIntentToBehaviorGoTo(t *testing.T) {
	deps := BehaviorDeps{
		GoTo: func(x, y, z int, sprint bool) BehaviorFunc {
			return func(BehaviorCtx) error { return nil }
		},
	}
	fn, channels, priority, err := MapIntentToBehavior(Intent{
		Action: "go_to",
		Params: map[string]any{"x": 1, "y": 64, "z": 2, "sprint": true},
	}, deps)
	if err != nil {
		t.Fatalf("MapIntentToBehavior error: %v", err)
	}
	if fn == nil {
		t.Fatal("expected behavior func")
	}
	if priority != 30 {
		t.Fatalf("priority=%d want 30", priority)
	}
	if len(channels) != 2 || channels[0] != ChannelLegs || channels[1] != ChannelHead {
		t.Fatalf("channels=%v", channels)
	}
}

func TestMapIntentToBehaviorLookAtEntity(t *testing.T) {
	deps := BehaviorDeps{
		LookAtEntity: func(entityID int32) BehaviorFunc {
			return func(BehaviorCtx) error { return nil }
		},
	}
	_, channels, priority, err := MapIntentToBehavior(Intent{
		Action: "look_at",
		Params: map[string]any{"entity_id": 42},
	}, deps)
	if err != nil {
		t.Fatalf("MapIntentToBehavior error: %v", err)
	}
	if priority != 30 {
		t.Fatalf("priority=%d want 30", priority)
	}
	if len(channels) != 1 || channels[0] != ChannelHead {
		t.Fatalf("channels=%v", channels)
	}
}

func TestMapIntentToBehaviorUnknownAction(t *testing.T) {
	_, _, _, err := MapIntentToBehavior(Intent{Action: "unknown", Params: map[string]any{}}, BehaviorDeps{})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestMapIntentToBehaviorMissingRequiredParam(t *testing.T) {
	deps := BehaviorDeps{
		Attack: func(entityID int32) BehaviorFunc {
			return func(BehaviorCtx) error { return nil }
		},
	}
	_, _, _, err := MapIntentToBehavior(Intent{Action: "attack", Params: map[string]any{}}, deps)
	if err == nil {
		t.Fatal("expected error for missing required field")
	}
}
