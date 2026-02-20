package agent

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Versifine/locus/internal/skill"
)

type Intent = skill.Intent

func ParseIntent(input map[string]any) (Intent, error) {
	if input == nil {
		return Intent{}, fmt.Errorf("intent input is nil")
	}
	action := strings.ToLower(strings.TrimSpace(asString(input["action"])))
	if action == "" {
		return Intent{}, fmt.Errorf("set_intent missing action")
	}

	params := make(map[string]any)
	switch action {
	case "idle":
		// no required params
	case "go_to":
		if err := requireIntParam(input, params, "x"); err != nil {
			return Intent{}, err
		}
		if err := requireIntParam(input, params, "y"); err != nil {
			return Intent{}, err
		}
		if err := requireIntParam(input, params, "z"); err != nil {
			return Intent{}, err
		}
		if v, ok := input["sprint"]; ok {
			if b, ok := asBool(v); ok {
				params["sprint"] = b
			}
		}
	case "follow":
		if err := requireIntParam(input, params, "entity_id"); err != nil {
			return Intent{}, err
		}
		if v, ok := input["distance"]; ok {
			if f, ok := asFloat64(v); ok {
				params["distance"] = f
			}
		}
		if v, ok := input["sprint"]; ok {
			if b, ok := asBool(v); ok {
				params["sprint"] = b
			}
		}
	case "look_at":
		if _, ok := input["entity_id"]; ok {
			if err := requireIntParam(input, params, "entity_id"); err != nil {
				return Intent{}, err
			}
		} else {
			if err := requireIntParam(input, params, "x"); err != nil {
				return Intent{}, err
			}
			if err := requireIntParam(input, params, "y"); err != nil {
				return Intent{}, err
			}
			if err := requireIntParam(input, params, "z"); err != nil {
				return Intent{}, err
			}
		}
	case "attack":
		if err := requireIntParam(input, params, "entity_id"); err != nil {
			return Intent{}, err
		}
	case "mine":
		if err := requireIntParam(input, params, "x"); err != nil {
			return Intent{}, err
		}
		if err := requireIntParam(input, params, "y"); err != nil {
			return Intent{}, err
		}
		if err := requireIntParam(input, params, "z"); err != nil {
			return Intent{}, err
		}
		if err := optionalSlot(input, params); err != nil {
			return Intent{}, err
		}
	case "place_block":
		if err := requireIntParam(input, params, "x"); err != nil {
			return Intent{}, err
		}
		if err := requireIntParam(input, params, "y"); err != nil {
			return Intent{}, err
		}
		if err := requireIntParam(input, params, "z"); err != nil {
			return Intent{}, err
		}
		if err := requireIntParam(input, params, "face"); err != nil {
			return Intent{}, err
		}
		if err := optionalSlot(input, params); err != nil {
			return Intent{}, err
		}
	case "use_item":
		if err := optionalSlot(input, params); err != nil {
			return Intent{}, err
		}
	case "switch_slot":
		if err := requireIntParam(input, params, "slot"); err != nil {
			return Intent{}, err
		}
		slot := params["slot"].(int)
		if slot < 0 || slot > 8 {
			return Intent{}, fmt.Errorf("slot out of range")
		}
	default:
		return Intent{}, fmt.Errorf("unknown intent action: %s", action)
	}

	if err := optionalDurationMs(input, params); err != nil {
		return Intent{}, err
	}

	return Intent{Action: action, Params: params}, nil
}

func parseIntent(input map[string]any) Intent {
	intent, _ := ParseIntent(input)
	return intent
}

func requireIntParam(src map[string]any, dst map[string]any, key string) error {
	value, ok := src[key]
	if !ok {
		return fmt.Errorf("missing %s", key)
	}
	if n, ok := asInt(value); ok {
		dst[key] = n
		return nil
	}
	return fmt.Errorf("invalid %s", key)
}

func optionalSlot(src map[string]any, dst map[string]any) error {
	v, ok := src["slot"]
	if !ok {
		return nil
	}
	n, ok := asInt(v)
	if !ok {
		return fmt.Errorf("invalid slot")
	}
	if n < 0 || n > 8 {
		return fmt.Errorf("slot out of range")
	}
	dst["slot"] = n
	return nil
}

func optionalDurationMs(src map[string]any, dst map[string]any) error {
	v, ok := src["duration_ms"]
	if !ok {
		return nil
	}
	n, ok := asInt(v)
	if !ok {
		return fmt.Errorf("invalid duration_ms")
	}
	if n < 0 {
		return fmt.Errorf("duration_ms out of range")
	}
	dst["duration_ms"] = n
	return nil
}

func asString(v any) string {
	s, _ := v.(string)
	return s
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

func asInt(v any) (int, bool) {
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
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
