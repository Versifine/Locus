package behaviors

import (
	"errors"

	"github.com/Versifine/locus/internal/skill"
)

const mineReachDistance = 4.5

func Mine(target skill.BlockPos, slot *int8) skill.BehaviorFunc {
	return func(bctx skill.BehaviorCtx) error {
		if bctx.Blocks == nil {
			return errors.New("mine requires block access")
		}

		snap := bctx.Snapshot()
		nav := newPathNavigator(32, 1.0)
		slotSent := false

		for {
			if isAirAt(bctx.Blocks, target) {
				return nil
			}

			lookYaw, lookPitch := skill.CalcLookAt(snap.Position, blockTopCenter(target))
			partial := skill.PartialInput{}
			if slot != nil && !slotSent {
				partial.HotbarSlot = int8Ptr(*slot)
				slotSent = true
			}

			if skill.IsNear(snap.Position, blockCenter(target), mineReachDistance) {
				partial.Yaw = float32Ptr(lookYaw)
				partial.Pitch = float32Ptr(lookPitch)
				partial.Attack = boolPtr(true)
				partial.BreakTarget = blockPosPtr(target)
			} else {
				approach, ok := nearestApproach(target, snap.Position, bctx.Blocks)
				if !ok {
					return errors.New("mine approach not found")
				}

				move, _, err := nav.Tick(snap, approach, bctx.Blocks, true)
				if err != nil {
					return err
				}
				partial.Forward = move.Forward
				partial.Yaw = move.Yaw
				partial.Sprint = move.Sprint
			}

			next, ok := skill.Step(bctx, partial)
			if !ok {
				return nil
			}
			snap = next
		}
	}
}
