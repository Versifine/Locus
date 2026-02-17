package behaviors

import (
	"errors"

	"github.com/Versifine/locus/internal/skill"
	"github.com/Versifine/locus/internal/world"
)

func Follow(entityID int32, distance float64) skill.BehaviorFunc {
	if distance <= 0 {
		distance = 3
	}

	return func(bctx skill.BehaviorCtx) error {
		if bctx.Blocks == nil {
			return errors.New("follow requires block access")
		}

		snap := bctx.Snapshot()
		nav := newPathNavigator(48, 1.0)
		var lastApproach skill.BlockPos
		hasLastApproach := false

		for {
			entity := skill.FindEntity(snap, entityID)
			if entity == nil {
				return nil
			}

			target := skill.Vec3{X: entity.X, Y: entity.Y, Z: entity.Z}
			yaw, pitch := skill.CalcLookAt(snap.Position, target)
			inRange := skill.IsNear(snap.Position, target, distance)

			partial := skill.PartialInput{
				Yaw:   float32Ptr(yaw),
				Pitch: float32Ptr(pitch),
			}

			if !inRange {
				targetBlock := toBlockPos(world.Position{X: entity.X, Y: entity.Y, Z: entity.Z})
				approach := targetBlock
				if near, ok := nearestApproach(targetBlock, snap.Position, bctx.Blocks); ok {
					approach = near
				}

				if !hasLastApproach || approach != lastApproach {
					nav.Invalidate()
					lastApproach = approach
					hasLastApproach = true
				}

				move, _, err := nav.Tick(snap, approach, bctx.Blocks, false)
				if err != nil {
					return err
				}
				partial.Forward = move.Forward
				if move.Yaw != nil {
					partial.Yaw = move.Yaw
				}
			} else {
				nav.Invalidate()
				hasLastApproach = false
			}

			next, ok := skill.Step(bctx, partial)
			if !ok {
				return nil
			}
			snap = next
		}
	}
}
