package skill

import (
	"fmt"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/world"
)

func TestBehaviorRunnerEmitsCompletedEnd(t *testing.T) {
	runner := NewBehaviorRunner(nil, nil, nil)
	if !runner.Start("done", func(BehaviorCtx) error { return nil }, nil, 1) {
		t.Fatal("start done failed")
	}

	evt := mustReadBehaviorEnd(t, runner.BehaviorEnds())
	if evt.Name != "done" {
		t.Fatalf("name=%q want done", evt.Name)
	}
	if evt.Reason != BehaviorExitCompleted {
		t.Fatalf("reason=%q want %q", evt.Reason, BehaviorExitCompleted)
	}
	if evt.RunID == 0 {
		t.Fatal("expected non-zero run id")
	}
}

func TestBehaviorRunnerEmitsCancelledEnd(t *testing.T) {
	runner := NewBehaviorRunner(nil, nil, nil)
	loop := func(bctx BehaviorCtx) error {
		for {
			_, ok := Step(bctx, PartialInput{})
			if !ok {
				return nil
			}
		}
	}
	if !runner.Start("loop", loop, []Channel{ChannelLegs}, 1) {
		t.Fatal("start loop failed")
	}
	runner.Tick(world.Snapshot{})
	runner.Cancel("loop")

	evt := mustReadBehaviorEnd(t, runner.BehaviorEnds())
	if evt.Name != "loop" {
		t.Fatalf("name=%q want loop", evt.Name)
	}
	if evt.Reason != BehaviorExitCancelled {
		t.Fatalf("reason=%q want %q", evt.Reason, BehaviorExitCancelled)
	}
}

func TestBehaviorRunnerEmitsPreemptedEnd(t *testing.T) {
	runner := NewBehaviorRunner(nil, nil, nil)
	loop := func(bctx BehaviorCtx) error {
		for {
			_, ok := Step(bctx, PartialInput{})
			if !ok {
				return nil
			}
		}
	}

	if !runner.Start("low", loop, []Channel{ChannelLegs}, 10) {
		t.Fatal("start low failed")
	}
	runner.Tick(world.Snapshot{})
	if !runner.Start("high", loop, []Channel{ChannelLegs}, 80) {
		t.Fatal("start high failed")
	}

	evt := mustReadBehaviorEnd(t, runner.BehaviorEnds())
	if evt.Name != "low" {
		t.Fatalf("name=%q want low", evt.Name)
	}
	if evt.Reason != BehaviorExitPreempted {
		t.Fatalf("reason=%q want %q", evt.Reason, BehaviorExitPreempted)
	}

	runner.CancelAll()
}

func TestBehaviorRunnerRunIDIncreases(t *testing.T) {
	runner := NewBehaviorRunner(nil, nil, nil)
	if !runner.Start("job", func(BehaviorCtx) error { return nil }, nil, 1) {
		t.Fatal("first start failed")
	}
	first := mustReadBehaviorEnd(t, runner.BehaviorEnds())

	if !runner.Start("job", func(BehaviorCtx) error { return nil }, nil, 1) {
		t.Fatal("second start failed")
	}
	second := mustReadBehaviorEnd(t, runner.BehaviorEnds())

	if second.RunID <= first.RunID {
		t.Fatalf("run ids not increasing: first=%d second=%d", first.RunID, second.RunID)
	}
}

func TestBehaviorRunnerBehaviorEndNoDropUnderBurst(t *testing.T) {
	runner := NewBehaviorRunner(nil, nil, nil)
	const total = 96

	for i := 0; i < total; i++ {
		name := fmt.Sprintf("job-%d", i)
		if !runner.Start(name, func(BehaviorCtx) error { return nil }, nil, 1) {
			t.Fatalf("start %s failed", name)
		}
	}

	seen := make(map[string]int)
	deadline := time.After(2 * time.Second)
	for len(seen) < total {
		select {
		case evt := <-runner.BehaviorEnds():
			seen[evt.Name]++
		case <-deadline:
			t.Fatalf("received %d/%d behavior end events", len(seen), total)
		}
	}

	for name, count := range seen {
		if count != 1 {
			t.Fatalf("event count for %s=%d, want 1", name, count)
		}
	}
}

func mustReadBehaviorEnd(t *testing.T, ch <-chan BehaviorEnd) BehaviorEnd {
	t.Helper()
	select {
	case evt := <-ch:
		return evt
	case <-time.After(time.Second):
		t.Fatal("timeout waiting behavior end event")
		return BehaviorEnd{}
	}
}
