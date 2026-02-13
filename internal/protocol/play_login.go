package protocol

import (
	"fmt"
	"io"
)

type GlobalPos struct {
	DimensionName string
	X             int32
	Y             int32
	Z             int32
}

type SpawnInfo struct {
	Dimension        int32
	Name             string
	HashedSeed       int64
	Gamemode         int8
	PreviousGamemode uint8
	IsDebug          bool
	IsFlat           bool
	Death            *GlobalPos
	PortalCooldown   int32
	SeaLevel         int32
}

type PlayLogin struct {
	EntityID            int32
	IsHardcore          bool
	WorldNames          []string
	MaxPlayers          int32
	ViewDistance        int32
	SimulationDistance  int32
	ReducedDebugInfo    bool
	EnableRespawnScreen bool
	DoLimitedCrafting   bool
	WorldState          SpawnInfo
	EnforcesSecureChat  bool
}

type Respawn struct {
	WorldState   SpawnInfo
	CopyMetadata uint8
}

func ParsePlayLogin(r io.Reader) (*PlayLogin, error) {
	entityID, err := ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read entity id: %w", err)
	}

	isHardcore, err := ReadBool(r)
	if err != nil {
		return nil, fmt.Errorf("read isHardcore: %w", err)
	}

	worldNameCount, err := ReadVarint(r)
	if err != nil {
		return nil, fmt.Errorf("read world names count: %w", err)
	}
	if worldNameCount < 0 {
		return nil, fmt.Errorf("invalid world names count: %d", worldNameCount)
	}
	worldNames := make([]string, 0, worldNameCount)
	for i := int32(0); i < worldNameCount; i++ {
		name, err := ReadString(r)
		if err != nil {
			return nil, fmt.Errorf("read world name %d: %w", i, err)
		}
		worldNames = append(worldNames, name)
	}

	maxPlayers, err := ReadVarint(r)
	if err != nil {
		return nil, fmt.Errorf("read maxPlayers: %w", err)
	}
	viewDistance, err := ReadVarint(r)
	if err != nil {
		return nil, fmt.Errorf("read viewDistance: %w", err)
	}
	simulationDistance, err := ReadVarint(r)
	if err != nil {
		return nil, fmt.Errorf("read simulationDistance: %w", err)
	}
	if maxPlayers < 0 || viewDistance < 0 || simulationDistance < 0 {
		return nil, fmt.Errorf(
			"invalid login limits: maxPlayers=%d viewDistance=%d simulationDistance=%d",
			maxPlayers,
			viewDistance,
			simulationDistance,
		)
	}

	reducedDebugInfo, err := ReadBool(r)
	if err != nil {
		return nil, fmt.Errorf("read reducedDebugInfo: %w", err)
	}
	enableRespawnScreen, err := ReadBool(r)
	if err != nil {
		return nil, fmt.Errorf("read enableRespawnScreen: %w", err)
	}
	doLimitedCrafting, err := ReadBool(r)
	if err != nil {
		return nil, fmt.Errorf("read doLimitedCrafting: %w", err)
	}

	worldState, err := parseSpawnInfo(r)
	if err != nil {
		return nil, fmt.Errorf("read worldState: %w", err)
	}

	enforcesSecureChat, err := ReadBool(r)
	if err != nil {
		return nil, fmt.Errorf("read enforcesSecureChat: %w", err)
	}

	return &PlayLogin{
		EntityID:            entityID,
		IsHardcore:          isHardcore,
		WorldNames:          worldNames,
		MaxPlayers:          maxPlayers,
		ViewDistance:        viewDistance,
		SimulationDistance:  simulationDistance,
		ReducedDebugInfo:    reducedDebugInfo,
		EnableRespawnScreen: enableRespawnScreen,
		DoLimitedCrafting:   doLimitedCrafting,
		WorldState:          worldState,
		EnforcesSecureChat:  enforcesSecureChat,
	}, nil
}

func ParseRespawn(r io.Reader) (*Respawn, error) {
	worldState, err := parseSpawnInfo(r)
	if err != nil {
		return nil, fmt.Errorf("read worldState: %w", err)
	}

	copyMetadata, err := ReadByte(r)
	if err != nil {
		return nil, fmt.Errorf("read copyMetadata: %w", err)
	}

	return &Respawn{
		WorldState:   worldState,
		CopyMetadata: copyMetadata,
	}, nil
}

func parseSpawnInfo(r io.Reader) (SpawnInfo, error) {
	dimension, err := ReadVarint(r)
	if err != nil {
		return SpawnInfo{}, fmt.Errorf("read dimension: %w", err)
	}

	name, err := ReadString(r)
	if err != nil {
		return SpawnInfo{}, fmt.Errorf("read world name: %w", err)
	}

	hashedSeed, err := ReadInt64(r)
	if err != nil {
		return SpawnInfo{}, fmt.Errorf("read hashedSeed: %w", err)
	}

	gamemodeByte, err := ReadByte(r)
	if err != nil {
		return SpawnInfo{}, fmt.Errorf("read gamemode: %w", err)
	}

	previousGamemode, err := ReadByte(r)
	if err != nil {
		return SpawnInfo{}, fmt.Errorf("read previousGamemode: %w", err)
	}

	isDebug, err := ReadBool(r)
	if err != nil {
		return SpawnInfo{}, fmt.Errorf("read isDebug: %w", err)
	}

	isFlat, err := ReadBool(r)
	if err != nil {
		return SpawnInfo{}, fmt.Errorf("read isFlat: %w", err)
	}

	hasDeath, err := ReadBool(r)
	if err != nil {
		return SpawnInfo{}, fmt.Errorf("read death present flag: %w", err)
	}

	var death *GlobalPos
	if hasDeath {
		dimensionName, err := ReadString(r)
		if err != nil {
			return SpawnInfo{}, fmt.Errorf("read death dimension name: %w", err)
		}
		x, y, z, err := readPackedBlockPosition(r)
		if err != nil {
			return SpawnInfo{}, fmt.Errorf("read death position: %w", err)
		}
		death = &GlobalPos{
			DimensionName: dimensionName,
			X:             x,
			Y:             y,
			Z:             z,
		}
	}

	portalCooldown, err := ReadVarint(r)
	if err != nil {
		return SpawnInfo{}, fmt.Errorf("read portalCooldown: %w", err)
	}
	seaLevel, err := ReadVarint(r)
	if err != nil {
		return SpawnInfo{}, fmt.Errorf("read seaLevel: %w", err)
	}

	return SpawnInfo{
		Dimension:        dimension,
		Name:             name,
		HashedSeed:       hashedSeed,
		Gamemode:         int8(gamemodeByte),
		PreviousGamemode: previousGamemode,
		IsDebug:          isDebug,
		IsFlat:           isFlat,
		Death:            death,
		PortalCooldown:   portalCooldown,
		SeaLevel:         seaLevel,
	}, nil
}
