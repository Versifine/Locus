package behaviors

import (
	"errors"

	"github.com/Versifine/locus/internal/skill"
	"github.com/Versifine/locus/internal/world"
)

const (
	attackRange         = 2.8
	attackCooldownTicks = 10
)

func Attack(entityID int32) skill.BehaviorFunc {
	return func(bctx skill.BehaviorCtx) error {
		if bctx.Blocks == nil {
			return errors.New("attack requires block access")
		}

		snap := bctx.Snapshot()
		ticks := 0
		lastAttackTick := -attackCooldownTicks
		nav := newPathNavigator(32, 1.0)
		var lastApproach skill.BlockPos
		hasLastApproach := false

		for {
			ticks++
			entity := skill.FindEntity(snap, entityID)
			if entity == nil {
				return nil
			}

			target := skill.Vec3{X: entity.X, Y: entity.Y + 0.9, Z: entity.Z}
			yaw, pitch := skill.CalcLookAt(snap.Position, target)
			inRange := skill.IsNear(snap.Position, target, attackRange)
			hasLOS := raycastClear(bctx.Blocks, eyePos(snap.Position), target, nil)

			partial := skill.PartialInput{
				Yaw:   float32Ptr(yaw),
				Pitch: float32Ptr(pitch),
			}
			if !inRange || !hasLOS {
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

				move, _, err := nav.Tick(snap, approach, bctx.Blocks, true)
				if err != nil {
					return err
				}
				partial.Forward = move.Forward
				partial.Jump = move.Jump
				partial.Sprint = move.Sprint
				if move.Yaw != nil {
					partial.Yaw = move.Yaw
				}
			} else {
				nav.Invalidate()
				hasLastApproach = false
			}

			if inRange && hasLOS && ticks-lastAttackTick >= attackCooldownTicks {
				partial.Attack = boolPtr(true)
				partial.AttackTarget = int32Ptr(entityID)
				lastAttackTick = ticks
			}

			next, ok := skill.Step(bctx, partial)
			if !ok {
				return nil
			}
			snap = next
		}
	}
}
