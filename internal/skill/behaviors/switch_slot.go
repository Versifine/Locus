package behaviors

import "github.com/Versifine/locus/internal/skill"

func SwitchSlot(slot int8, durationMs int) skill.BehaviorFunc {
	return func(bctx skill.BehaviorCtx) error {
		_ = durationMs
		_, _ = skill.Step(bctx, skill.PartialInput{HotbarSlot: int8Ptr(slot)})
		return nil
	}
}
