package agent

import (
	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/world"
)

type Attention struct {
	prevSnap      world.Snapshot
	hasPrevSnap   bool
	bus           *event.Bus
	SpatialMemory *SpatialMemory
}

func NewAttention(bus *event.Bus) *Attention {
	return &Attention{bus: bus}
}

func (a *Attention) Tick(snap world.Snapshot, tick uint64) {
	if a == nil {
		return
	}
	if !a.hasPrevSnap {
		if a.SpatialMemory != nil && len(snap.Entities) > 0 {
			a.SpatialMemory.UpdateEntities(snap.Entities, tick)
			a.SpatialMemory.GC()
		}
		a.prevSnap = snap
		a.hasPrevSnap = true
		return
	}

	if a.bus != nil && snap.Health < a.prevSnap.Health {
		a.bus.Publish(event.EventDamage, event.DamageEvent{
			Amount: a.prevSnap.Health - snap.Health,
			NewHP:  snap.Health,
		})
	}

	prevMap := make(map[int32]world.Entity, len(a.prevSnap.Entities))
	for _, entity := range a.prevSnap.Entities {
		prevMap[entity.EntityID] = entity
	}
	if a.SpatialMemory != nil && len(snap.Entities) > 0 {
		a.SpatialMemory.UpdateEntities(snap.Entities, tick)
	}

	currMap := make(map[int32]world.Entity, len(snap.Entities))
	for _, entity := range snap.Entities {
		currMap[entity.EntityID] = entity
		if _, existed := prevMap[entity.EntityID]; !existed {
			if a.bus != nil {
				a.bus.Publish(event.EventEntityAppear, event.EntityEvent{
					EntityID: entity.EntityID,
					Name:     entityDisplayName(entity),
					Type:     entity.Type,
				})
			}
		}
	}

	for entityID, entity := range prevMap {
		if _, stillPresent := currMap[entityID]; stillPresent {
			continue
		}
		if a.SpatialMemory != nil {
			a.SpatialMemory.MarkEntityLeft(entity.EntityID, tick)
		}
		if a.bus != nil {
			a.bus.Publish(event.EventEntityLeave, event.EntityEvent{
				EntityID: entity.EntityID,
				Name:     entityDisplayName(entity),
				Type:     entity.Type,
			})
		}
	}
	if a.SpatialMemory != nil {
		a.SpatialMemory.GC()
	}

	a.prevSnap = snap
}

func entityDisplayName(entity world.Entity) string {
	if entity.Type == 71 && entity.ItemName != "" {
		return "Item(" + entity.ItemName + ")"
	}
	if name := world.EntityTypeName(entity.Type); name != "" {
		return name
	}
	return "Unknown"
}
