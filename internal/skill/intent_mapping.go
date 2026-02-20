package skill

import "fmt"

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

type Intent struct {
	Action string
	Params map[string]any
}

type BehaviorDeps struct {
	Idle         func(durationMs int) BehaviorFunc
	GoTo         func(x, y, z int, sprint bool, durationMs int) BehaviorFunc
	Follow       func(entityID int32, distance float64, sprint bool, durationMs int) BehaviorFunc
	LookAtEntity func(entityID int32, durationMs int) BehaviorFunc
	LookAtPos    func(target Vec3, durationMs int) BehaviorFunc
	Attack       func(entityID int32, durationMs int) BehaviorFunc
	Mine         func(pos BlockPos, slot *int8, durationMs int) BehaviorFunc
	PlaceBlock   func(pos BlockPos, face int, slot *int8, durationMs int) BehaviorFunc
	UseItem      func(slot *int8, durationMs int) BehaviorFunc
	SwitchSlot   func(slot int8, durationMs int) BehaviorFunc
}

func MapIntentToBehavior(intent Intent, deps BehaviorDeps) (BehaviorFunc, []Channel, int, error) {
	durationMs := optionalDurationMs(intent.Params)

	switch intent.Action {
	case "idle":
		if deps.Idle == nil {
			return nil, nil, 0, fmt.Errorf("idle behavior factory is nil")
		}
		return deps.Idle(durationMs), []Channel{ChannelLegs, ChannelHead}, PriorityIdle, nil
	case "go_to":
		if deps.GoTo == nil {
			return nil, nil, 0, fmt.Errorf("go_to behavior factory is nil")
		}
		x, err := asInt(intent.Params, "x")
		if err != nil {
			return nil, nil, 0, err
		}
		y, err := asInt(intent.Params, "y")
		if err != nil {
			return nil, nil, 0, err
		}
		z, err := asInt(intent.Params, "z")
		if err != nil {
			return nil, nil, 0, err
		}
		sprint, _ := asBool(intent.Params["sprint"])
		return deps.GoTo(x, y, z, sprint, durationMs), []Channel{ChannelLegs, ChannelHead}, PriorityGoTo, nil
	case "follow":
		if deps.Follow == nil {
			return nil, nil, 0, fmt.Errorf("follow behavior factory is nil")
		}
		entityID, err := asInt32(intent.Params, "entity_id")
		if err != nil {
			return nil, nil, 0, err
		}
		distance, ok := asFloat64(intent.Params["distance"])
		if !ok || distance <= 0 {
			distance = 3
		}
		sprint, _ := asBool(intent.Params["sprint"])
		return deps.Follow(entityID, distance, sprint, durationMs), []Channel{ChannelLegs, ChannelHead}, PriorityFollow, nil
	case "look_at":
		if entityRaw, ok := intent.Params["entity_id"]; ok {
			if deps.LookAtEntity == nil {
				return nil, nil, 0, fmt.Errorf("look_at(entity) behavior factory is nil")
			}
			entityID, ok := asInt32FromAny(entityRaw)
			if !ok {
				return nil, nil, 0, fmt.Errorf("invalid entity_id")
			}
			return deps.LookAtEntity(entityID, durationMs), []Channel{ChannelHead}, PriorityLookAt, nil
		}
		if deps.LookAtPos == nil {
			return nil, nil, 0, fmt.Errorf("look_at(pos) behavior factory is nil")
		}
		x, err := asInt(intent.Params, "x")
		if err != nil {
			return nil, nil, 0, err
		}
		y, err := asInt(intent.Params, "y")
		if err != nil {
			return nil, nil, 0, err
		}
		z, err := asInt(intent.Params, "z")
		if err != nil {
			return nil, nil, 0, err
		}
		target := Vec3{X: float64(x), Y: float64(y), Z: float64(z)}
		return deps.LookAtPos(target, durationMs), []Channel{ChannelHead}, PriorityLookAt, nil
	case "attack":
		if deps.Attack == nil {
			return nil, nil, 0, fmt.Errorf("attack behavior factory is nil")
		}
		entityID, err := asInt32(intent.Params, "entity_id")
		if err != nil {
			return nil, nil, 0, err
		}
		return deps.Attack(entityID, durationMs), []Channel{ChannelLegs, ChannelHead, ChannelHands}, PriorityAttack, nil
	case "mine":
		if deps.Mine == nil {
			return nil, nil, 0, fmt.Errorf("mine behavior factory is nil")
		}
		x, err := asInt(intent.Params, "x")
		if err != nil {
			return nil, nil, 0, err
		}
		y, err := asInt(intent.Params, "y")
		if err != nil {
			return nil, nil, 0, err
		}
		z, err := asInt(intent.Params, "z")
		if err != nil {
			return nil, nil, 0, err
		}
		slot := optionalSlot(intent.Params)
		return deps.Mine(BlockPos{X: x, Y: y, Z: z}, slot, durationMs), []Channel{ChannelLegs, ChannelHead, ChannelHands}, PriorityMine, nil
	case "place_block":
		if deps.PlaceBlock == nil {
			return nil, nil, 0, fmt.Errorf("place_block behavior factory is nil")
		}
		x, err := asInt(intent.Params, "x")
		if err != nil {
			return nil, nil, 0, err
		}
		y, err := asInt(intent.Params, "y")
		if err != nil {
			return nil, nil, 0, err
		}
		z, err := asInt(intent.Params, "z")
		if err != nil {
			return nil, nil, 0, err
		}
		face, err := asInt(intent.Params, "face")
		if err != nil {
			return nil, nil, 0, err
		}
		slot := optionalSlot(intent.Params)
		return deps.PlaceBlock(BlockPos{X: x, Y: y, Z: z}, face, slot, durationMs), []Channel{ChannelLegs, ChannelHead, ChannelHands}, PriorityPlaceBlock, nil
	case "use_item":
		if deps.UseItem == nil {
			return nil, nil, 0, fmt.Errorf("use_item behavior factory is nil")
		}
		slot := optionalSlot(intent.Params)
		return deps.UseItem(slot, durationMs), []Channel{ChannelHands}, PriorityUseItem, nil
	case "switch_slot":
		if deps.SwitchSlot == nil {
			return nil, nil, 0, fmt.Errorf("switch_slot behavior factory is nil")
		}
		slot, err := asInt(intent.Params, "slot")
		if err != nil {
			return nil, nil, 0, err
		}
		return deps.SwitchSlot(int8(slot), durationMs), []Channel{ChannelHands}, PrioritySwitchSlot, nil
	default:
		return nil, nil, 0, fmt.Errorf("unknown intent action: %s", intent.Action)
	}
}

func asInt(params map[string]any, key string) (int, error) {
	if params == nil {
		return 0, fmt.Errorf("missing %s", key)
	}
	v, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	if out, ok := asIntFromAny(v); ok {
		return out, nil
	}
	return 0, fmt.Errorf("invalid %s", key)
}

func asInt32(params map[string]any, key string) (int32, error) {
	v, err := asInt(params, key)
	if err != nil {
		return 0, err
	}
	return int32(v), nil
}

func optionalSlot(params map[string]any) *int8 {
	if params == nil {
		return nil
	}
	v, ok := params["slot"]
	if !ok {
		return nil
	}
	slotInt, ok := asIntFromAny(v)
	if !ok {
		return nil
	}
	s := int8(slotInt)
	return &s
}

func optionalDurationMs(params map[string]any) int {
	if params == nil {
		return 0
	}
	v, ok := params["duration_ms"]
	if !ok {
		return 0
	}
	d, ok := asIntFromAny(v)
	if !ok || d <= 0 {
		return 0
	}
	return d
}

func asInt32FromAny(v any) (int32, bool) {
	i, ok := asIntFromAny(v)
	if !ok {
		return 0, false
	}
	return int32(i), true
}

func asIntFromAny(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	default:
		return 0, false
	}
}

func asBool(v any) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}

func asFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
