package skill

import (
	"context"

	"github.com/Versifine/locus/internal/world"
)

type BehaviorCtx struct {
	ctx      context.Context
	cancel   context.CancelFunc
	tick     <-chan world.Snapshot
	output   chan<- PartialInput
	send     func(string) error
	snapshot func() world.Snapshot
}

func (b BehaviorCtx) Send(message string) error {
	if b.send == nil {
		return nil
	}
	return b.send(message)
}

func (b BehaviorCtx) Snapshot() world.Snapshot {
	if b.snapshot == nil {
		return world.Snapshot{}
	}
	return b.snapshot()
}

func (b BehaviorCtx) Done() <-chan struct{} {
	if b.ctx == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return b.ctx.Done()
}

func (b BehaviorCtx) Cancel() {
	if b.cancel != nil {
		b.cancel()
	}
}

func step(bctx BehaviorCtx, input PartialInput) (world.Snapshot, bool) {
	if bctx.ctx == nil {
		return world.Snapshot{}, false
	}

	select {
	case <-bctx.ctx.Done():
		return world.Snapshot{}, false
	case bctx.output <- input:
	}

	select {
	case <-bctx.ctx.Done():
		return world.Snapshot{}, false
	case snap, ok := <-bctx.tick:
		if !ok {
			return world.Snapshot{}, false
		}
		return snap, true
	}
}
