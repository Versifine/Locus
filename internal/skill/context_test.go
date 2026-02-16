package skill

import (
	"context"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/world"
)

func TestStepPushesInputAndReceivesSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tickCh := make(chan world.Snapshot, 1)
	outCh := make(chan PartialInput, 1)
	bctx := BehaviorCtx{ctx: ctx, cancel: cancel, tick: tickCh, output: outCh}

	forward := true
	wantInput := PartialInput{Forward: &forward}

	done := make(chan struct{})
	var (
		gotSnap world.Snapshot
		ok      bool
	)
	go func() {
		gotSnap, ok = step(bctx, wantInput)
		close(done)
	}()

	select {
	case got := <-outCh:
		if got.Forward == nil || !*got.Forward {
			t.Fatalf("got forward = %v, want true", got.Forward)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting output")
	}

	tickCh <- world.Snapshot{Health: 18}

	select {
	case <-done:
		if !ok {
			t.Fatal("step returned ok=false")
		}
		if gotSnap.Health != 18 {
			t.Fatalf("snapshot health = %.1f, want 18", gotSnap.Health)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting step result")
	}
}

func TestStepReturnsFalseOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tickCh := make(chan world.Snapshot)
	outCh := make(chan PartialInput, 1)
	bctx := BehaviorCtx{ctx: ctx, cancel: cancel, tick: tickCh, output: outCh}

	forward := true
	done := make(chan bool, 1)
	go func() {
		_, ok := step(bctx, PartialInput{Forward: &forward})
		done <- ok
	}()

	<-outCh
	cancel()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("ok=true, want false")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting cancel result")
	}
}
