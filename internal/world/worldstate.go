package world

import (
	"fmt"
	"strings"
	"sync"
)

type WorldState struct {
	position   Position
	health     float32
	food       int32
	gameTime   GameTime
	playerList []Player
	mu         sync.RWMutex
}

type Snapshot struct {
	Position   Position
	Health     float32
	Food       int32
	GameTime   GameTime
	PlayerList []Player
}

func (s Snapshot) String() string {
	var playerInfos []string
	for _, p := range s.PlayerList {
		playerInfos = append(playerInfos, fmt.Sprintf("Username: %s UUID: (%s)", p.Name, p.UUID))
	}
	playerInfosStr := fmt.Sprintf("[%s]", strings.Join(playerInfos, ", "))

	timeOfDay := s.GameTime.WorldTime % 24000
	hours := (timeOfDay/1000 + 6) % 24
	minutes := (timeOfDay % 1000) * 60 / 1000

	return fmt.Sprintf(
		"Snapshot [Time: %02d:%02d] | [Position: (X: %.2f, Y: %.2f, Z: %.2f, Yaw: %.2f, Pitch: %.2f)] | [Health: %.2f] | [Food: %d] | [Players: %s]",
		hours, minutes,
		s.Position.X, s.Position.Y, s.Position.Z, s.Position.Yaw, s.Position.Pitch,
		s.Health,
		s.Food,
		playerInfosStr,
	)
}

func (ws *WorldState) GetState() Snapshot {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	// Return a copy of the world state
	return Snapshot{
		Position:   ws.position,
		Health:     ws.health,
		Food:       ws.food,
		GameTime:   ws.gameTime,
		PlayerList: append([]Player(nil), ws.playerList...),
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

type Player struct {
	Name string
	UUID string
}

func (ws *WorldState) AddPlayer(players []Player) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.playerList = append(ws.playerList, players...)
}

func (ws *WorldState) RemovePlayer(uuid string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	for i, player := range ws.playerList {
		if player.UUID == uuid {
			ws.playerList = append(ws.playerList[:i], ws.playerList[i+1:]...)
			break
		}
	}
}
