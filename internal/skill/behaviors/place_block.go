package behaviors

import (
	"errors"

	"github.com/Versifine/locus/internal/skill"
)

const (
	placeReachDistance      = 4.5
	placeConfirmTimeoutTick = 80
	placeRetryIntervalTick  = 8
)

func PlaceBlock(target skill.BlockPos, face int, slot *int8) skill.BehaviorFunc {
	return func(bctx skill.BehaviorCtx) error {
		if bctx.Blocks == nil {
			return errors.New("place_block requires block access")
		}

		snap := bctx.Snapshot()
		nav := newPathNavigator(32, 1.0)
		slotSent := false
		waitingConfirm := false
		confirmTicks := 0
		retryCooldown := 0

		for {
			if !isAirAt(bctx.Blocks, target) {
				return nil
			}

			partial := skill.PartialInput{}
			if slot != nil && !slotSent {
				partial.HotbarSlot = int8Ptr(*slot)
				slotSent = true
			}

			if skill.IsNear(snap.Position, blockCenter(target), placeReachDistance) {
				yaw, pitch := skill.CalcLookAt(snap.Position, blockTopCenter(target))
				partial.Yaw = float32Ptr(yaw)
				partial.Pitch = float32Ptr(pitch)

				if retryCooldown > 0 {
					retryCooldown--
				}
				if retryCooldown == 0 {
					partial.Use = boolPtr(true)
					partial.PlaceTarget = placeActionPtr(target, face)
					waitingConfirm = true
					retryCooldown = placeRetryIntervalTick
				}
				if waitingConfirm {
					confirmTicks++
					if confirmTicks > placeConfirmTimeoutTick {
						return errors.New("place block confirmation timeout")
					}
				}
			} else {
				approach, ok := nearestApproach(target, snap.Position, bctx.Blocks)
				if !ok {
					return errors.New("place block approach not found")
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
