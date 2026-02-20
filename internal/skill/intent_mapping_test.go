package skill

import "testing"

func TestMapIntentToBehaviorGoTo(t *testing.T) {
	called := false
	deps := BehaviorDeps{
		GoTo: func(x, y, z int, sprint bool, durationMs int) BehaviorFunc {
			called = true
			if x != 1 || y != 64 || z != 2 {
				t.Fatalf("coords=(%d,%d,%d) want (1,64,2)", x, y, z)
			}
			if !sprint {
				t.Fatal("expected sprint=true")
			}
			if durationMs != 150 {
				t.Fatalf("durationMs=%d want 150", durationMs)
			}
			return func(BehaviorCtx) error { return nil }
		},
	}
	fn, channels, priority, err := MapIntentToBehavior(Intent{
		Action: "go_to",
		Params: map[string]any{"x": 1, "y": 64, "z": 2, "sprint": true, "duration_ms": 150},
	}, deps)
	if err != nil {
		t.Fatalf("MapIntentToBehavior error: %v", err)
	}
	if !called {
		t.Fatal("expected go_to factory to be called")
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

func TestMapIntentToBehaviorFollowSprint(t *testing.T) {
	called := false
	deps := BehaviorDeps{
		Follow: func(entityID int32, distance float64, sprint bool, durationMs int) BehaviorFunc {
			called = true
			if entityID != 42 {
				t.Fatalf("entityID=%d want 42", entityID)
			}
			if distance != 4.5 {
				t.Fatalf("distance=%v want 4.5", distance)
			}
			if !sprint {
				t.Fatal("expected sprint=true")
			}
			if durationMs != 250 {
				t.Fatalf("durationMs=%d want 250", durationMs)
			}
			return func(BehaviorCtx) error { return nil }
		},
	}

	_, channels, priority, err := MapIntentToBehavior(Intent{
		Action: "follow",
		Params: map[string]any{"entity_id": 42, "distance": 4.5, "sprint": true, "duration_ms": 250},
	}, deps)
	if err != nil {
		t.Fatalf("MapIntentToBehavior error: %v", err)
	}
	if !called {
		t.Fatal("expected follow factory to be called")
	}
	if priority != PriorityFollow {
		t.Fatalf("priority=%d want %d", priority, PriorityFollow)
	}
	if len(channels) != 2 || channels[0] != ChannelLegs || channels[1] != ChannelHead {
		t.Fatalf("channels=%v", channels)
	}
}

func TestMapIntentToBehaviorLookAtEntity(t *testing.T) {
	called := false
	deps := BehaviorDeps{
		LookAtEntity: func(entityID int32, durationMs int) BehaviorFunc {
			called = true
			if entityID != 42 {
				t.Fatalf("entityID=%d want 42", entityID)
			}
			if durationMs != 80 {
				t.Fatalf("durationMs=%d want 80", durationMs)
			}
			return func(BehaviorCtx) error { return nil }
		},
	}
	_, channels, priority, err := MapIntentToBehavior(Intent{
		Action: "look_at",
		Params: map[string]any{"entity_id": 42, "duration_ms": 80},
	}, deps)
	if err != nil {
		t.Fatalf("MapIntentToBehavior error: %v", err)
	}
	if !called {
		t.Fatal("expected look_at(entity) factory call")
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
		Attack: func(entityID int32, durationMs int) BehaviorFunc {
			return func(BehaviorCtx) error { return nil }
		},
	}
	_, _, _, err := MapIntentToBehavior(Intent{Action: "attack", Params: map[string]any{}}, deps)
	if err == nil {
		t.Fatal("expected error for missing required field")
	}
}
