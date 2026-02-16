package skill

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/world"
)

func TestBehaviorRunnerTickMergeAndChannels(t *testing.T) {
	runner := NewBehaviorRunner(nil, nil)

	legsStarted := make(chan struct{}, 1)
	handsStarted := make(chan struct{}, 1)

	legsFn := func(bctx BehaviorCtx) error {
		forward := true
		yaw := float32(90)
		legsStarted <- struct{}{}
		for {
			_, ok := step(bctx, PartialInput{Forward: &forward, Yaw: &yaw})
			if !ok {
				return nil
			}
		}
	}
	handsFn := func(bctx BehaviorCtx) error {
		attack := true
		handsStarted <- struct{}{}
		for {
			_, ok := step(bctx, PartialInput{Attack: &attack})
			if !ok {
				return nil
			}
		}
	}

	if !runner.Start("legs", legsFn, []Channel{ChannelLegs, ChannelHead}, 10) {
		t.Fatal("start legs failed")
	}
	if !runner.Start("hands", handsFn, []Channel{ChannelHands}, 10) {
		t.Fatal("start hands failed")
	}

	waitSignal(t, legsStarted)
	waitSignal(t, handsStarted)

	var out world.Position
	var input any
	_ = out
	_ = input

	var got struct {
		Forward bool
		Yaw     float32
		Attack  bool
	}

	eventually(t, time.Second, func() bool {
		in := runner.Tick(world.Snapshot{})
		got.Forward = in.Forward
		got.Yaw = in.Yaw
		got.Attack = in.Attack
		return got.Forward && got.Attack && got.Yaw == 90
	})

	runner.CancelAll()
}

func TestBehaviorRunnerPriorityPreempt(t *testing.T) {
	runner := NewBehaviorRunner(nil, nil)
	var lowCanceled atomic.Bool

	lowFn := func(bctx BehaviorCtx) error {
		forward := true
		for {
			_, ok := step(bctx, PartialInput{Forward: &forward})
			if !ok {
				lowCanceled.Store(true)
				return nil
			}
		}
	}
	highFn := func(bctx BehaviorCtx) error {
		back := true
		for {
			_, ok := step(bctx, PartialInput{Backward: &back})
			if !ok {
				return nil
			}
		}
	}

	if !runner.Start("low", lowFn, []Channel{ChannelLegs}, 10) {
		t.Fatal("start low failed")
	}
	if !runner.Start("high", highFn, []Channel{ChannelLegs}, 20) {
		t.Fatal("start high should preempt low")
	}
	eventually(t, time.Second, func() bool { return lowCanceled.Load() })

	if runner.Start("low2", lowFn, []Channel{ChannelLegs}, 10) {
		t.Fatal("low2 should be rejected by high")
	}

	runner.CancelAll()
}

func TestBehaviorRunnerCancelAndActive(t *testing.T) {
	runner := NewBehaviorRunner(nil, nil)

	noop := func(bctx BehaviorCtx) error {
		for {
			_, ok := step(bctx, PartialInput{})
			if !ok {
				return nil
			}
		}
	}

	if !runner.Start("a", noop, []Channel{ChannelLegs}, 1) {
		t.Fatal("start a failed")
	}
	if !runner.Start("b", noop, []Channel{ChannelHands}, 1) {
		t.Fatal("start b failed")
	}

	eventually(t, time.Second, func() bool { return runner.ActiveCount() == 2 })

	runner.Cancel("a")
	eventually(t, time.Second, func() bool { return runner.ActiveCount() == 1 })

	active := runner.Active()
	if len(active) != 1 || active[0] != "b" {
		t.Fatalf("active=%v, want [b]", active)
	}

	runner.CancelAll()
	eventually(t, time.Second, func() bool { return runner.ActiveCount() == 0 })
}

func waitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting signal")
	}
}

func eventually(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}
