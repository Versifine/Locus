package behaviors

import (
	"math/rand"
	"time"

	"github.com/Versifine/locus/internal/skill"
)

func Idle() skill.BehaviorFunc {
	return func(bctx skill.BehaviorCtx) error {
		snap := bctx.Snapshot()
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		yawTarget := snap.Position.Yaw
		pitchTarget := snap.Position.Pitch
		walkTicks := 0

		for {
			if walkTicks == 0 && rng.Intn(20) == 0 {
				walkTicks = 2 + rng.Intn(4)
			}

			if rng.Intn(6) == 0 {
				yawTarget = normalizeYaw(snap.Position.Yaw + float32(rng.Float64()*16.0-8.0))
				pitchTarget = clampPitch(snap.Position.Pitch + float32(rng.Float64()*6.0-3.0))
			}

			forward := walkTicks > 0
			if walkTicks > 0 {
				walkTicks--
			}

			partial := skill.PartialInput{
				Forward: boolPtr(forward),
				Yaw:     float32Ptr(yawTarget),
				Pitch:   float32Ptr(pitchTarget),
			}
			next, ok := skill.Step(bctx, partial)
			if !ok {
				return nil
			}
			snap = next
		}
	}
}
