package behaviors

import "github.com/Versifine/locus/internal/skill"

func UseItem(slot *int8, durationMs int) skill.BehaviorFunc {
	return func(bctx skill.BehaviorCtx) error {
		slotSent := false
		timedOut := durationCheck(durationMs)

		for {
			partial := skill.PartialInput{Use: boolPtr(true)}
			if slot != nil && !slotSent {
				partial.HotbarSlot = int8Ptr(*slot)
				slotSent = true
			}

			_, ok := skill.Step(bctx, partial)
			if !ok {
				return nil
			}
			if timedOut() {
				return nil
			}
		}
	}
}
