package skill

import (
	"context"

	"github.com/Versifine/locus/internal/world"
)

type BlockAccess interface {
	GetBlockState(x, y, z int) (int32, bool)
	GetBlockNameByStateID(stateID int32) (string, bool)
	IsSolid(x, y, z int) bool
}

type BehaviorCtx struct {
	Ctx        context.Context
	CancelFunc context.CancelFunc
	Tick       <-chan world.Snapshot
	Output     chan<- PartialInput
	SendFunc   func(string) error
	SnapshotFn func() world.Snapshot
	Blocks     BlockAccess
}

func (b BehaviorCtx) Send(message string) error {
	if b.SendFunc == nil {
		return nil
	}
	return b.SendFunc(message)
}

func (b BehaviorCtx) Snapshot() world.Snapshot {
	if b.SnapshotFn == nil {
		return world.Snapshot{}
	}
	return b.SnapshotFn()
}

func (b BehaviorCtx) Done() <-chan struct{} {
	if b.Ctx == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return b.Ctx.Done()
}

func (b BehaviorCtx) Cancel() {
	if b.CancelFunc != nil {
		b.CancelFunc()
	}
}

func Step(bctx BehaviorCtx, input PartialInput) (world.Snapshot, bool) {
	if bctx.Ctx == nil {
		return world.Snapshot{}, false
	}

	select {
	case <-bctx.Ctx.Done():
		return world.Snapshot{}, false
	case bctx.Output <- input:
	}

	select {
	case <-bctx.Ctx.Done():
		return world.Snapshot{}, false
	case snap, ok := <-bctx.Tick:
		if !ok {
			return world.Snapshot{}, false
		}
		return snap, true
	}
}

func step(bctx BehaviorCtx, input PartialInput) (world.Snapshot, bool) {
	return Step(bctx, input)
}
