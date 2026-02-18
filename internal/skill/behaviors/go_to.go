package behaviors

import (
	"errors"

	"github.com/Versifine/locus/internal/skill"
)

func GoTo(x, y, z int, sprint bool) skill.BehaviorFunc {
	target := skill.BlockPos{X: x, Y: y, Z: z}

	return func(bctx skill.BehaviorCtx) error {
		if bctx.Blocks == nil {
			return errors.New("go_to requires block access")
		}

		snap := bctx.Snapshot()
		nav := newPathNavigator(64, defaultNearDist)

		for {
			partial, done, err := nav.Tick(snap, target, bctx.Blocks, sprint)
			if err != nil {
				return err
			}
			if done {
				return nil
			}

			next, ok := skill.Step(bctx, partial)
			if !ok {
				return nil
			}
			snap = next
		}
	}
}
