package behaviors

import "github.com/Versifine/locus/internal/skill"

func Deps() skill.BehaviorDeps {
	return skill.BehaviorDeps{
		Idle:         Idle,
		GoTo:         GoTo,
		Follow:       Follow,
		LookAtEntity: LookAtEntity,
		LookAtPos:    LookAtPos,
		Attack:       Attack,
		Mine:         Mine,
		PlaceBlock:   PlaceBlock,
		UseItem:      UseItem,
		SwitchSlot:   SwitchSlot,
	}
}
