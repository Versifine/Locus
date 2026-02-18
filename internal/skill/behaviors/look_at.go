package behaviors

import "github.com/Versifine/locus/internal/skill"

func LookAtEntity(entityID int32) skill.BehaviorFunc {
	return func(bctx skill.BehaviorCtx) error {
		snap := bctx.Snapshot()
		for {
			entity := skill.FindEntity(snap, entityID)
			if entity == nil {
				return nil
			}

			target := skill.Vec3{X: entity.X, Y: entity.Y, Z: entity.Z}
			yaw, pitch := skill.CalcLookAt(snap.Position, target)

			next, ok := skill.Step(bctx, skill.PartialInput{
				Yaw:   float32Ptr(yaw),
				Pitch: float32Ptr(pitch),
			})
			if !ok {
				return nil
			}
			snap = next
		}
	}
}

func LookAtPos(target skill.Vec3) skill.BehaviorFunc {
	return func(bctx skill.BehaviorCtx) error {
		snap := bctx.Snapshot()
		for {
			yaw, pitch := skill.CalcLookAt(snap.Position, target)
			if absf64(float64(skill.AngleDiff(snap.Position.Yaw, yaw))) <= lookAlignedThreshold &&
				absf64(float64(pitch-snap.Position.Pitch)) <= lookAlignedThreshold {
				return nil
			}

			next, ok := skill.Step(bctx, skill.PartialInput{
				Yaw:   float32Ptr(yaw),
				Pitch: float32Ptr(pitch),
			})
			if !ok {
				return nil
			}
			snap = next
		}
	}
}
