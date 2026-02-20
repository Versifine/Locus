package agent

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Versifine/locus/internal/world"
)

const (
	defaultSpatialMemoryEntityMaxAge = 60 * time.Second
	defaultSpatialSummaryMaxAge      = 30 * time.Second
	defaultSpatialQueryRadius        = 16.0
	maxSpatialMemoryBlocks           = 4096
	spatialSummaryEntityLimit        = 6
	spatialSummaryBlockLimit         = 6
)

type SpatialMemory struct {
	mu           sync.RWMutex
	entities     map[int32]EntityMemory
	blocks       map[[3]int]BlockMemory
	maxEntityAge time.Duration
}

type EntityMemory struct {
	EntityID int32
	Type     int32
	Name     string
	X        float64
	Y        float64
	Z        float64
	LastSeen time.Time
	TickID   uint64
	InFOV    bool
}

type BlockMemory struct {
	Name     string
	Pos      [3]int
	LastSeen time.Time
	TickID   uint64
}

func NewSpatialMemory() *SpatialMemory {
	return &SpatialMemory{
		entities:     make(map[int32]EntityMemory),
		blocks:       make(map[[3]int]BlockMemory),
		maxEntityAge: defaultSpatialMemoryEntityMaxAge,
	}
}

func (m *SpatialMemory) UpdateEntities(entities []world.Entity, tick uint64) {
	if m == nil || len(entities) == 0 {
		return
	}
	now := time.Now()

	m.mu.Lock()
	if m.entities == nil {
		m.entities = make(map[int32]EntityMemory, len(entities))
	}
	for _, entity := range entities {
		m.entities[entity.EntityID] = EntityMemory{
			EntityID: entity.EntityID,
			Type:     entity.Type,
			Name:     spatialEntityName(entity),
			X:        entity.X,
			Y:        entity.Y,
			Z:        entity.Z,
			LastSeen: now,
			TickID:   tick,
			InFOV:    true,
		}
	}
	m.mu.Unlock()
}

func (m *SpatialMemory) UpdateBlocks(blocks []BlockInfo, tick uint64) {
	if m == nil || len(blocks) == 0 {
		return
	}
	now := time.Now()

	m.mu.Lock()
	if m.blocks == nil {
		m.blocks = make(map[[3]int]BlockMemory, len(blocks))
	}
	for _, block := range blocks {
		name := strings.TrimSpace(block.Type)
		if name == "" {
			continue
		}
		m.blocks[block.Pos] = BlockMemory{
			Name:     name,
			Pos:      block.Pos,
			LastSeen: now,
			TickID:   tick,
		}
	}
	m.mu.Unlock()
}

func (m *SpatialMemory) MarkEntityLeft(entityID int32, tick uint64) {
	if m == nil || entityID == 0 {
		return
	}
	m.mu.Lock()
	if mem, ok := m.entities[entityID]; ok {
		mem.InFOV = false
		mem.TickID = tick
		m.entities[entityID] = mem
	}
	m.mu.Unlock()
}

func (m *SpatialMemory) QueryNearby(center Vec3, radius float64, maxAge time.Duration) ([]EntityMemory, []BlockMemory) {
	if m == nil {
		return nil, nil
	}
	if radius <= 0 {
		radius = defaultSpatialQueryRadius
	}
	if maxAge <= 0 {
		maxAge = defaultSpatialSummaryMaxAge
	}
	maxAge = max(maxAge, time.Second)

	now := time.Now()
	maxDistSq := radius * radius

	m.mu.RLock()
	entities := make([]EntityMemory, 0, len(m.entities))
	for _, memory := range m.entities {
		if now.Sub(memory.LastSeen) > maxAge {
			continue
		}
		dx := memory.X - center.X
		dy := memory.Y - center.Y
		dz := memory.Z - center.Z
		if dx*dx+dy*dy+dz*dz > maxDistSq {
			continue
		}
		entities = append(entities, memory)
	}

	blocks := make([]BlockMemory, 0, len(m.blocks))
	for _, memory := range m.blocks {
		if now.Sub(memory.LastSeen) > maxAge {
			continue
		}
		dx := float64(memory.Pos[0]) + 0.5 - center.X
		dy := float64(memory.Pos[1]) + 0.5 - center.Y
		dz := float64(memory.Pos[2]) + 0.5 - center.Z
		if dx*dx+dy*dy+dz*dz > maxDistSq {
			continue
		}
		blocks = append(blocks, memory)
	}
	m.mu.RUnlock()

	sort.Slice(entities, func(i, j int) bool {
		di := sqDistVec(center, Vec3{X: entities[i].X, Y: entities[i].Y, Z: entities[i].Z})
		dj := sqDistVec(center, Vec3{X: entities[j].X, Y: entities[j].Y, Z: entities[j].Z})
		if di != dj {
			return di < dj
		}
		if entities[i].TickID != entities[j].TickID {
			return entities[i].TickID > entities[j].TickID
		}
		return entities[i].EntityID < entities[j].EntityID
	})

	sort.Slice(blocks, func(i, j int) bool {
		di := sqDistVec(center, Vec3{X: float64(blocks[i].Pos[0]) + 0.5, Y: float64(blocks[i].Pos[1]) + 0.5, Z: float64(blocks[i].Pos[2]) + 0.5})
		dj := sqDistVec(center, Vec3{X: float64(blocks[j].Pos[0]) + 0.5, Y: float64(blocks[j].Pos[1]) + 0.5, Z: float64(blocks[j].Pos[2]) + 0.5})
		if di != dj {
			return di < dj
		}
		if blocks[i].TickID != blocks[j].TickID {
			return blocks[i].TickID > blocks[j].TickID
		}
		if blocks[i].Name != blocks[j].Name {
			return blocks[i].Name < blocks[j].Name
		}
		if blocks[i].Pos[1] != blocks[j].Pos[1] {
			return blocks[i].Pos[1] < blocks[j].Pos[1]
		}
		if blocks[i].Pos[0] != blocks[j].Pos[0] {
			return blocks[i].Pos[0] < blocks[j].Pos[0]
		}
		return blocks[i].Pos[2] < blocks[j].Pos[2]
	})

	return entities, blocks
}

func (m *SpatialMemory) Summary(center Vec3, radius float64) string {
	entities, blocks := m.QueryNearby(center, radius, defaultSpatialSummaryMaxAge)
	return spatialSummaryFromMemories(entities, blocks, defaultSpatialSummaryMaxAge)
}

func spatialSummaryFromMemories(entities []EntityMemory, blocks []BlockMemory, maxAge time.Duration) string {
	now := time.Now()
	ageLabel := spatialAgeLabel(maxAge)

	entityText := "none"
	if len(entities) > 0 {
		limit := len(entities)
		if limit > spatialSummaryEntityLimit {
			limit = spatialSummaryEntityLimit
		}
		parts := make([]string, 0, limit+1)
		for i := 0; i < limit; i++ {
			entity := entities[i]
			ageSec := int(now.Sub(entity.LastSeen).Seconds())
			if ageSec < 0 {
				ageSec = 0
			}
			state := "memory"
			if entity.InFOV {
				state = "visible"
			}
			parts = append(parts, fmt.Sprintf("%s(id=%d,%s) at [%d,%d,%d] %ds ago", strings.TrimSpace(entity.Name), entity.EntityID, state, int(math.Round(entity.X)), int(math.Round(entity.Y)), int(math.Round(entity.Z)), ageSec))
		}
		if len(entities) > limit {
			parts = append(parts, fmt.Sprintf("+%d more", len(entities)-limit))
		}
		entityText = strings.Join(parts, "; ")
	}

	blockText := "none"
	if len(blocks) > 0 {
		type blockGroup struct {
			name  string
			count int
			min   [3]int
			max   [3]int
		}

		groups := make(map[string]blockGroup)
		for _, block := range blocks {
			group, ok := groups[block.Name]
			if !ok {
				group = blockGroup{name: block.Name, count: 0, min: block.Pos, max: block.Pos}
			}
			group.count++
			if block.Pos[0] < group.min[0] {
				group.min[0] = block.Pos[0]
			}
			if block.Pos[1] < group.min[1] {
				group.min[1] = block.Pos[1]
			}
			if block.Pos[2] < group.min[2] {
				group.min[2] = block.Pos[2]
			}
			if block.Pos[0] > group.max[0] {
				group.max[0] = block.Pos[0]
			}
			if block.Pos[1] > group.max[1] {
				group.max[1] = block.Pos[1]
			}
			if block.Pos[2] > group.max[2] {
				group.max[2] = block.Pos[2]
			}
			groups[block.Name] = group
		}

		ordered := make([]blockGroup, 0, len(groups))
		for _, group := range groups {
			ordered = append(ordered, group)
		}
		sort.Slice(ordered, func(i, j int) bool {
			if ordered[i].count != ordered[j].count {
				return ordered[i].count > ordered[j].count
			}
			return ordered[i].name < ordered[j].name
		})

		limit := len(ordered)
		if limit > spatialSummaryBlockLimit {
			limit = spatialSummaryBlockLimit
		}
		parts := make([]string, 0, limit+1)
		for i := 0; i < limit; i++ {
			group := ordered[i]
			if group.count == 1 {
				parts = append(parts, fmt.Sprintf("%s at [%d,%d,%d]", group.name, group.min[0], group.min[1], group.min[2]))
				continue
			}
			parts = append(parts, fmt.Sprintf("%s x%d in [%s,%s,%s]", group.name, group.count, rangeString(group.min[0], group.max[0]), rangeString(group.min[1], group.max[1]), rangeString(group.min[2], group.max[2])))
		}
		if len(ordered) > limit {
			parts = append(parts, fmt.Sprintf("+%d more", len(ordered)-limit))
		}
		blockText = strings.Join(parts, "; ")
	}

	return fmt.Sprintf("Nearby entities (last %s): %s\nRecent blocks: %s", ageLabel, entityText, blockText)
}

func (m *SpatialMemory) GC() {
	if m == nil {
		return
	}
	now := time.Now()

	m.mu.Lock()
	maxAge := m.maxEntityAge
	if maxAge <= 0 {
		maxAge = defaultSpatialMemoryEntityMaxAge
	}
	for id, entity := range m.entities {
		if now.Sub(entity.LastSeen) > maxAge {
			delete(m.entities, id)
		}
	}

	if len(m.blocks) <= maxSpatialMemoryBlocks {
		m.mu.Unlock()
		return
	}

	type blockEntry struct {
		key      [3]int
		lastSeen time.Time
		tickID   uint64
	}

	entries := make([]blockEntry, 0, len(m.blocks))
	for key, block := range m.blocks {
		entries = append(entries, blockEntry{key: key, lastSeen: block.LastSeen, tickID: block.TickID})
	}
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].lastSeen.Equal(entries[j].lastSeen) {
			return entries[i].lastSeen.Before(entries[j].lastSeen)
		}
		if entries[i].tickID != entries[j].tickID {
			return entries[i].tickID < entries[j].tickID
		}
		if entries[i].key[1] != entries[j].key[1] {
			return entries[i].key[1] < entries[j].key[1]
		}
		if entries[i].key[0] != entries[j].key[0] {
			return entries[i].key[0] < entries[j].key[0]
		}
		return entries[i].key[2] < entries[j].key[2]
	})

	remove := len(entries) - maxSpatialMemoryBlocks
	for i := 0; i < remove; i++ {
		delete(m.blocks, entries[i].key)
	}
	m.mu.Unlock()
}

func spatialEntityName(entity world.Entity) string {
	if entity.Type == 71 && strings.TrimSpace(entity.ItemName) != "" {
		return "Item(" + strings.TrimSpace(entity.ItemName) + ")"
	}
	if name := strings.TrimSpace(world.EntityTypeName(entity.Type)); name != "" {
		return name
	}
	return fmt.Sprintf("Unknown(%d)", entity.Type)
}

func sqDistVec(a, b Vec3) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return dx*dx + dy*dy + dz*dz
}

func spatialAgeLabel(maxAge time.Duration) string {
	if maxAge <= 0 {
		maxAge = defaultSpatialSummaryMaxAge
	}
	seconds := int(math.Round(maxAge.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return fmt.Sprintf("%ds", seconds)
}
