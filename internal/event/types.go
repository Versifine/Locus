package event

const (
	EventDamage       = "damage"
	EventBehaviorEnd  = "behavior.end"
	EventEntityAppear = "entity.appear"
	EventEntityLeave  = "entity.leave"
)

type DamageEvent struct {
	Amount float32
	NewHP  float32
}

type BehaviorEndEvent struct {
	Name   string
	RunID  uint64
	Reason string
}

type EntityEvent struct {
	EntityID int32
	Name     string
	Type     int32
}
