package behaviors

import "github.com/Versifine/locus/internal/skill"

type Spec struct {
	Name     string
	Fn       skill.BehaviorFunc
	Channels []skill.Channel
	Priority int
}

const (
	PriorityIdle       = 10
	PriorityGoTo       = 30
	PriorityFollow     = 40
	PriorityLookAt     = 30
	PriorityAttack     = 80
	PriorityMine       = 40
	PriorityPlaceBlock = 30
	PriorityUseItem    = 30
	PrioritySwitchSlot = 50
)

func IdleSpec(durationMs int) Spec {
	return Spec{
		Name:     "idle",
		Fn:       Idle(durationMs),
		Channels: []skill.Channel{skill.ChannelLegs, skill.ChannelHead},
		Priority: PriorityIdle,
	}
}

func GoToSpec(x, y, z int, sprint bool, durationMs int) Spec {
	return Spec{
		Name:     "go_to",
		Fn:       GoTo(x, y, z, sprint, durationMs),
		Channels: []skill.Channel{skill.ChannelLegs, skill.ChannelHead},
		Priority: PriorityGoTo,
	}
}

func FollowSpec(entityID int32, distance float64, sprint bool, durationMs int) Spec {
	return Spec{
		Name:     "follow",
		Fn:       Follow(entityID, distance, sprint, durationMs),
		Channels: []skill.Channel{skill.ChannelLegs, skill.ChannelHead},
		Priority: PriorityFollow,
	}
}

func LookAtEntitySpec(entityID int32, durationMs int) Spec {
	return Spec{
		Name:     "look_at",
		Fn:       LookAtEntity(entityID, durationMs),
		Channels: []skill.Channel{skill.ChannelHead},
		Priority: PriorityLookAt,
	}
}

func LookAtPosSpec(target skill.Vec3, durationMs int) Spec {
	return Spec{
		Name:     "look_at",
		Fn:       LookAtPos(target, durationMs),
		Channels: []skill.Channel{skill.ChannelHead},
		Priority: PriorityLookAt,
	}
}

func AttackSpec(entityID int32, durationMs int) Spec {
	return Spec{
		Name:     "attack",
		Fn:       Attack(entityID, durationMs),
		Channels: []skill.Channel{skill.ChannelLegs, skill.ChannelHead, skill.ChannelHands},
		Priority: PriorityAttack,
	}
}

func MineSpec(pos skill.BlockPos, slot *int8, durationMs int) Spec {
	return Spec{
		Name:     "mine",
		Fn:       Mine(pos, slot, durationMs),
		Channels: []skill.Channel{skill.ChannelLegs, skill.ChannelHead, skill.ChannelHands},
		Priority: PriorityMine,
	}
}

func PlaceBlockSpec(pos skill.BlockPos, face int, slot *int8, durationMs int) Spec {
	return Spec{
		Name:     "place_block",
		Fn:       PlaceBlock(pos, face, slot, durationMs),
		Channels: []skill.Channel{skill.ChannelLegs, skill.ChannelHead, skill.ChannelHands},
		Priority: PriorityPlaceBlock,
	}
}

func UseItemSpec(slot *int8, durationMs int) Spec {
	return Spec{
		Name:     "use_item",
		Fn:       UseItem(slot, durationMs),
		Channels: []skill.Channel{skill.ChannelHands},
		Priority: PriorityUseItem,
	}
}

func SwitchSlotSpec(slot int8, durationMs int) Spec {
	return Spec{
		Name:     "switch_slot",
		Fn:       SwitchSlot(slot, durationMs),
		Channels: []skill.Channel{skill.ChannelHands},
		Priority: PrioritySwitchSlot,
	}
}
