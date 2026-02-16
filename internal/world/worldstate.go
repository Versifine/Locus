package world

import (
	"fmt"
	"math"
	"strings"
	"sync"
)

type WorldState struct {
	position         Position
	health           float32
	food             int32
	gameTime         GameTime
	dimensionName    string
	simulationDist   int32
	viewCenterChunkX int32
	viewCenterChunkZ int32
	playerList       []Player
	entities         map[int32]*Entity
	pendingItemNames map[int32]string
	mu               sync.RWMutex
}

type Entity struct {
	EntityID int32
	UUID     string
	Type     int32
	X        float64
	Y        float64
	Z        float64
	ItemName string
}

type Snapshot struct {
	Position           Position
	Health             float32
	Food               int32
	GameTime           GameTime
	DimensionName      string
	SimulationDistance int32
	ViewCenterChunkX   int32
	ViewCenterChunkZ   int32
	PlayerList         []Player
	Entities           []Entity
}

func (s Snapshot) String() string {
	var playerInfos []string
	for _, p := range s.PlayerList {
		playerInfos = append(playerInfos, fmt.Sprintf("Username: %s UUID: (%s)", p.Name, p.UUID))
	}
	playerInfosStr := fmt.Sprintf("[%s]", strings.Join(playerInfos, ", "))

	// Build UUID→PlayerName lookup for entity cross-referencing
	uuidToName := make(map[string]string, len(s.PlayerList))
	for _, p := range s.PlayerList {
		uuidToName[p.UUID] = p.Name
	}

	var entityInfos []string
	for _, e := range s.Entities {
		dist := math.Sqrt(
			(e.X-s.Position.X)*(e.X-s.Position.X) +
				(e.Y-s.Position.Y)*(e.Y-s.Position.Y) +
				(e.Z-s.Position.Z)*(e.Z-s.Position.Z),
		)
		if name, ok := uuidToName[e.UUID]; ok {
			entityInfos = append(entityInfos, fmt.Sprintf("Player:%s ID:%d (%.1f, %.1f, %.1f) dist:%.1f", name, e.EntityID, e.X, e.Y, e.Z, dist))
		} else {
			typeName := EntityTypeName(e.Type)
			if e.Type == 71 && e.ItemName != "" {
				typeName = fmt.Sprintf("Item(%s)", e.ItemName)
			}
			if typeName == "" {
				typeName = fmt.Sprintf("Unknown(%d)", e.Type)
			}
			entityInfos = append(entityInfos, fmt.Sprintf("%s ID:%d (%.1f, %.1f, %.1f) dist:%.1f", typeName, e.EntityID, e.X, e.Y, e.Z, dist))
		}
	}
	entitiesStr := fmt.Sprintf("[%s]", strings.Join(entityInfos, ", "))

	timeOfDay := s.GameTime.WorldTime % 24000
	hours := (timeOfDay/1000 + 6) % 24
	minutes := (timeOfDay % 1000) * 60 / 1000

	return fmt.Sprintf(
		"Snapshot [Time: %02d:%02d] | [Position: (X: %.2f, Y: %.2f, Z: %.2f, Yaw: %.2f, Pitch: %.2f)] | [Health: %.2f] | [Food: %d] | [Players: %s] | [Entities(%d): %s]",
		hours, minutes,
		s.Position.X, s.Position.Y, s.Position.Z, s.Position.Yaw, s.Position.Pitch,
		s.Health,
		s.Food,
		playerInfosStr,
		len(s.Entities),
		entitiesStr,
	)
}

func (ws *WorldState) GetState() Snapshot {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	entities := make([]Entity, 0, len(ws.entities))
	for _, e := range ws.entities {
		entities = append(entities, *e)
	}
	return Snapshot{
		Position:           ws.position,
		Health:             ws.health,
		Food:               ws.food,
		GameTime:           ws.gameTime,
		DimensionName:      ws.dimensionName,
		SimulationDistance: ws.simulationDist,
		ViewCenterChunkX:   ws.viewCenterChunkX,
		ViewCenterChunkZ:   ws.viewCenterChunkZ,
		PlayerList:         append([]Player(nil), ws.playerList...),
		Entities:           entities,
	}
}

type Position struct {
	X     float64
	Y     float64
	Z     float64
	Yaw   float32
	Pitch float32
}

func (ws *WorldState) UpdatePosition(pos Position) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.position = pos
}
func (ws *WorldState) UpdateHealth(health float32, food int32) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.health = health
	ws.food = food
}

type GameTime struct {
	WorldTime int64
	Age       int64
}

func (ws *WorldState) UpdateGameTime(gameTime GameTime) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.gameTime = gameTime
}

func (ws *WorldState) UpdateDimensionContext(dimensionName string, simulationDistance int32) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.dimensionName = dimensionName
	ws.simulationDist = simulationDistance
}

func (ws *WorldState) UpdateViewCenter(chunkX, chunkZ int32) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.viewCenterChunkX = chunkX
	ws.viewCenterChunkZ = chunkZ
}

type Player struct {
	Name string
	UUID string
}

func (ws *WorldState) AddPlayer(players []Player) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if len(players) == 0 {
		return
	}

	existing := make(map[string]int, len(ws.playerList))
	for i, player := range ws.playerList {
		existing[player.UUID] = i
	}

	for _, player := range players {
		if idx, ok := existing[player.UUID]; ok {
			// Keep player list unique by UUID and refresh latest name.
			ws.playerList[idx] = player
			continue
		}
		ws.playerList = append(ws.playerList, player)
		existing[player.UUID] = len(ws.playerList) - 1
	}
}

func (ws *WorldState) RemovePlayer(uuid string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if len(ws.playerList) == 0 {
		return
	}

	filtered := ws.playerList[:0]
	for _, player := range ws.playerList {
		if player.UUID != uuid {
			filtered = append(filtered, player)
		}
	}
	ws.playerList = filtered
}

func (ws *WorldState) AddEntity(e Entity) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if ws.entities == nil {
		ws.entities = make(map[int32]*Entity)
	}
	if ws.pendingItemNames == nil {
		ws.pendingItemNames = make(map[int32]string)
	}
	if pending, ok := ws.pendingItemNames[e.EntityID]; ok {
		if e.Type == 71 {
			e.ItemName = pending
		}
		delete(ws.pendingItemNames, e.EntityID)
	}
	ws.entities[e.EntityID] = &e
}

func (ws *WorldState) RemoveEntities(ids []int32) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	for _, id := range ids {
		delete(ws.entities, id)
		if ws.pendingItemNames != nil {
			delete(ws.pendingItemNames, id)
		}
	}
}

func (ws *WorldState) ClearEntities() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if len(ws.entities) > 0 {
		ws.entities = make(map[int32]*Entity)
	}
	if len(ws.pendingItemNames) > 0 {
		ws.pendingItemNames = make(map[int32]string)
	}
}

func (ws *WorldState) UpdateEntityPosition(entityID int32, x, y, z float64) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if e, ok := ws.entities[entityID]; ok {
		e.X = x
		e.Y = y
		e.Z = z
	}
}

func (ws *WorldState) UpdateEntityPositionRelative(entityID int32, dx, dy, dz float64) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if e, ok := ws.entities[entityID]; ok {
		e.X += dx
		e.Y += dy
		e.Z += dz
	}
}

func (ws *WorldState) UpdateEntityItemName(entityID int32, itemName string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if e, ok := ws.entities[entityID]; ok && e.Type == 71 {
		e.ItemName = itemName
		return
	}
	if ws.pendingItemNames == nil {
		ws.pendingItemNames = make(map[int32]string)
	}
	ws.pendingItemNames[entityID] = itemName
}
