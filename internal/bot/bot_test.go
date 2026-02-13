package bot

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/protocol"
	"github.com/Versifine/locus/internal/world"
)

func TestBotLoginAndConfig(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	bot := NewBot("localhost:25565", "TestBot")
	bot.conn = client
	bot.connState = protocol.NewConnState()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use a channel to report completion
	done := make(chan struct{})
	var botErr error

	go func() {
		defer close(done)
		// We can't call Start because it calls Dial.
		// Instead we manually call login and handleConfiguration.
		if err := bot.login(); err != nil {
			botErr = err
			return
		}
		if err := bot.handleConfiguration(); err != nil {
			botErr = err
			return
		}
	}()

	// Server-side simulation
	go func() {
		threshold := -1
		// 1. Read Handshake
		p, err := protocol.ReadPacket(server, threshold)
		if err != nil {
			t.Errorf("Server: failed to read handshake: %v", err)
			return
		}
		if p.ID != protocol.C2SHandshake {
			t.Errorf("Server: expected handshake, got %x", p.ID)
			return
		}

		// 2. Read Login Start
		p, err = protocol.ReadPacket(server, threshold)
		if err != nil {
			t.Errorf("Server: failed to read login start: %v", err)
			return
		}
		if p.ID != protocol.C2SLoginStart {
			t.Errorf("Server: expected login start, got %x", p.ID)
			return
		}

		// 3. Send Login Success
		successPacket := &protocol.Packet{
			ID: protocol.S2CLoginSuccess,
			Payload: func() []byte {
				buf := new(bytes.Buffer)
				_ = protocol.WriteUUID(buf, protocol.GenerateOfflineUUID("TestBot"))
				_ = protocol.WriteString(buf, "TestBot")
				_ = protocol.WriteVarint(buf, 0) // properties
				return buf.Bytes()
			}(),
		}
		if err := protocol.WritePacket(server, successPacket, threshold); err != nil {
			t.Errorf("Server: failed to send login success: %v", err)
			return
		}

		// 4. Read Login Acknowledged
		p, err = protocol.ReadPacket(server, threshold)
		if err != nil {
			t.Errorf("Server: failed to read login ack: %v", err)
			return
		}
		if p.ID != protocol.C2SLoginAcknowledged {
			t.Errorf("Server: expected login ack, got %x", p.ID)
			return
		}

		// 5. Read Client Information (Config start)
		p, err = protocol.ReadPacket(server, threshold)
		if err != nil {
			t.Errorf("Server: failed to read client info: %v", err)
			return
		}
		if p.ID != protocol.C2SConfigClientInformation {
			t.Errorf("Server: expected client info, got %x", p.ID)
			return
		}

		// 6. Send Finish Configuration
		finishPacket := &protocol.Packet{
			ID:      protocol.S2CFinishConfiguration,
			Payload: []byte{},
		}
		if err := protocol.WritePacket(server, finishPacket, threshold); err != nil {
			t.Errorf("Server: failed to send finish config: %v", err)
			return
		}

		// 7. Read Finish Configuration Ack
		p, err = protocol.ReadPacket(server, threshold)
		if err != nil {
			t.Errorf("Server: failed to read finish config ack: %v", err)
			return
		}
		if p.ID != protocol.C2SFinishConfiguration {
			t.Errorf("Server: expected finish config ack, got %x", p.ID)
			return
		}
	}()

	select {
	case <-done:
		if botErr != nil {
			t.Fatalf("Bot failed: %v", botErr)
		}
	case <-ctx.Done():
		t.Fatal("Test timed out")
	}
}

func TestHandleLevelChunkWithLightAndUnload(t *testing.T) {
	blockStore, err := world.NewBlockStore()
	if err != nil {
		t.Fatalf("NewBlockStore failed: %v", err)
	}

	bot := &Bot{
		worldState: &world.WorldState{},
		blockStore: blockStore,
	}
	bot.worldState.UpdatePosition(world.Position{X: 1.2, Y: 64.0, Z: 2.8})

	payload := buildChunkPacketPayload(t, 0, 0, map[int]int32{
		7: 1234, // section containing y=63 (under feet when y=64)
	})
	bot.handleLevelChunkWithLight(payload)

	if !bot.blockStore.IsLoaded(0, 0) {
		t.Fatalf("chunk (0,0) should be loaded")
	}
	if bot.blockStore.LoadedChunkCount() != 1 {
		t.Fatalf("LoadedChunkCount = %d, want 1", bot.blockStore.LoadedChunkCount())
	}

	state, ok := bot.GetBlockState(1, 63, 2)
	if !ok {
		t.Fatalf("GetBlockState(1,63,2) should return loaded block")
	}
	if state != 1234 {
		t.Fatalf("GetBlockState(1,63,2) = %d, want 1234", state)
	}

	unloadPayload := new(bytes.Buffer)
	_ = protocol.WriteInt32(unloadPayload, 0) // chunkZ first
	_ = protocol.WriteInt32(unloadPayload, 0) // chunkX
	bot.handleUnloadChunk(unloadPayload.Bytes())

	if bot.blockStore.IsLoaded(0, 0) {
		t.Fatalf("chunk (0,0) should be unloaded")
	}
	if bot.blockStore.LoadedChunkCount() != 0 {
		t.Fatalf("LoadedChunkCount = %d, want 0", bot.blockStore.LoadedChunkCount())
	}
}

type chunkBlockEntityPayload struct {
	LocalX byte
	LocalZ byte
	Y      int16
	TypeID int32
	NBTTag byte
	NBTInt int32
}

func buildChunkPacketPayload(t *testing.T, chunkX, chunkZ int32, sectionStates map[int]int32) []byte {
	return buildChunkPacketPayloadWithBlockEntities(t, chunkX, chunkZ, sectionStates, nil)
}

func buildChunkPacketPayloadWithBlockEntities(
	t *testing.T,
	chunkX, chunkZ int32,
	sectionStates map[int]int32,
	blockEntities []chunkBlockEntityPayload,
) []byte {
	t.Helper()

	chunkData := new(bytes.Buffer)
	for section := 0; section < protocol.ChunkSectionCount; section++ {
		stateID := int32(0)
		if v, ok := sectionStates[section]; ok {
			stateID = v
		}

		if err := binary.Write(chunkData, binary.BigEndian, int16(4096)); err != nil {
			t.Fatalf("write block count failed: %v", err)
		}
		_ = protocol.WriteByte(chunkData, 0)         // block states: single value
		_ = protocol.WriteVarint(chunkData, stateID) // block state id
		_ = protocol.WriteVarint(chunkData, 0)       // block states data array length
		_ = protocol.WriteByte(chunkData, 0)         // biomes: single value
		_ = protocol.WriteVarint(chunkData, 0)
		_ = protocol.WriteVarint(chunkData, 0) // biomes data array length
	}

	payload := new(bytes.Buffer)
	_ = protocol.WriteInt32(payload, chunkX)
	_ = protocol.WriteInt32(payload, chunkZ)
	_ = protocol.WriteVarint(payload, 0) // heightmaps array
	_ = protocol.WriteVarint(payload, int32(chunkData.Len()))
	_, _ = payload.Write(chunkData.Bytes())
	_ = protocol.WriteVarint(payload, int32(len(blockEntities)))
	for _, be := range blockEntities {
		_ = protocol.WriteByte(payload, byte((be.LocalX<<4)|(be.LocalZ&0x0F)))
		_ = binary.Write(payload, binary.BigEndian, be.Y)
		_ = protocol.WriteVarint(payload, be.TypeID)
		_ = protocol.WriteByte(payload, be.NBTTag)
		if be.NBTTag == protocol.TagInt {
			_ = protocol.WriteInt32(payload, be.NBTInt)
		}
	}
	for i := 0; i < 6; i++ { // light data arrays
		_ = protocol.WriteVarint(payload, 0)
	}

	return payload.Bytes()
}

func TestHandleLevelChunkWithLightStoresBlockEntities(t *testing.T) {
	blockStore, err := world.NewBlockStore()
	if err != nil {
		t.Fatalf("NewBlockStore failed: %v", err)
	}
	bot := &Bot{
		worldState: &world.WorldState{},
		blockStore: blockStore,
	}

	payload := buildChunkPacketPayloadWithBlockEntities(
		t,
		0,
		0,
		map[int]int32{7: 1234},
		[]chunkBlockEntityPayload{
			{
				LocalX: 5,
				LocalZ: 7,
				Y:      70,
				TypeID: 42,
				NBTTag: protocol.TagInt,
				NBTInt: 99,
			},
		},
	)
	bot.handleLevelChunkWithLight(payload)

	entity, ok := bot.blockStore.GetBlockEntity(5, 70, 7)
	if !ok {
		t.Fatalf("GetBlockEntity should return block entity from chunk payload")
	}
	if entity.TypeID != 42 {
		t.Fatalf("TypeID = %d, want 42", entity.TypeID)
	}

	nbtNode, ok := entity.NBTData.(*protocol.NBTNode)
	if !ok || nbtNode == nil {
		t.Fatalf("NBTData should be *protocol.NBTNode, got %+v", entity.NBTData)
	}
	if nbtNode.Type != protocol.TagInt {
		t.Fatalf("NBTData.Type = %d, want TagInt", nbtNode.Type)
	}
	if v, ok := nbtNode.Value.(int32); !ok || v != 99 {
		t.Fatalf("unexpected NBT value: %+v", nbtNode.Value)
	}
}

func TestNormalizeSectionsForBlockStore16Sections(t *testing.T) {
	parsed := make([]protocol.ChunkSection, 16)
	for i := range parsed {
		states := make([]int32, world.BlocksPerSection)
		for j := range states {
			states[j] = int32(500 + i)
		}
		parsed[i] = protocol.ChunkSection{
			BlockCount:  4096,
			BlockStates: states,
		}
	}

	normalized, err := normalizeSectionsForBlockStore(parsed)
	if err != nil {
		t.Fatalf("normalizeSectionsForBlockStore failed: %v", err)
	}
	if len(normalized) != world.ChunkSectionCount {
		t.Fatalf("len(normalized) = %d, want %d", len(normalized), world.ChunkSectionCount)
	}

	// 16 sections should be centered into 24 => offset 4.
	if normalized[4].BlockStates[0] != 500 {
		t.Fatalf("normalized[4].BlockStates[0] = %d, want 500", normalized[4].BlockStates[0])
	}
	if normalized[19].BlockStates[0] != 515 {
		t.Fatalf("normalized[19].BlockStates[0] = %d, want 515", normalized[19].BlockStates[0])
	}
	if normalized[0].BlockStates[0] != 0 {
		t.Fatalf("normalized[0].BlockStates[0] = %d, want 0 for padded section", normalized[0].BlockStates[0])
	}
	if normalized[23].BlockStates[0] != 0 {
		t.Fatalf("normalized[23].BlockStates[0] = %d, want 0 for padded section", normalized[23].BlockStates[0])
	}
}

func TestCaptureFailedChunkPayloadLimitAndFiles(t *testing.T) {
	captureDir := filepath.Join(t.TempDir(), "captures")
	b := &Bot{
		chunkCaptureDir: captureDir,
		chunkCaptureMax: 2,
	}

	payload := make([]byte, 16)
	var chunkX int32 = 12
	var chunkZ int32 = -34
	binary.BigEndian.PutUint32(payload[0:4], uint32(chunkX))
	binary.BigEndian.PutUint32(payload[4:8], uint32(chunkZ))

	b.captureFailedChunkPayload(payload, errors.New("parse fail #1"))
	files, err := os.ReadDir(captureDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("after first capture file count = %d, want 2 (.bin + .json)", len(files))
	}

	var hasBin bool
	var hasJSON bool
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".bin") {
			hasBin = true
		}
		if strings.HasSuffix(f.Name(), ".json") {
			hasJSON = true
		}
	}
	if !hasBin || !hasJSON {
		t.Fatalf("first capture should create both .bin and .json files")
	}

	b.captureFailedChunkPayload(payload, errors.New("parse fail #2"))
	files, err = os.ReadDir(captureDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != 4 {
		t.Fatalf("after second capture file count = %d, want 4", len(files))
	}

	b.captureFailedChunkPayload(payload, errors.New("parse fail #3"))
	files, err = os.ReadDir(captureDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != 4 {
		t.Fatalf("after third capture (over limit) file count = %d, want 4", len(files))
	}
}

func TestHandleBlockChangeUpdatesLoadedChunk(t *testing.T) {
	blockStore, err := world.NewBlockStore()
	if err != nil {
		t.Fatalf("NewBlockStore failed: %v", err)
	}
	bot := &Bot{
		worldState: &world.WorldState{},
		blockStore: blockStore,
	}

	payload := buildChunkPacketPayload(t, 0, 0, map[int]int32{
		7: 1234,
	})
	bot.handleLevelChunkWithLight(payload)

	before, ok := bot.GetBlockState(1, 63, 2)
	if !ok || before != 1234 {
		t.Fatalf("before block state = (%d,%v), want (1234,true)", before, ok)
	}

	blockChange := new(bytes.Buffer)
	_ = protocol.WriteInt64(blockChange, packBlockPosition(1, 63, 2))
	_ = protocol.WriteVarint(blockChange, 1)
	bot.handleBlockChange(blockChange.Bytes())

	after, ok := bot.GetBlockState(1, 63, 2)
	if !ok {
		t.Fatalf("GetBlockState should return ok=true after block change")
	}
	if after != 1 {
		t.Fatalf("after block state = %d, want 1", after)
	}
}

func TestHandleTileEntityDataUpdatesBlockEntity(t *testing.T) {
	blockStore, err := world.NewBlockStore()
	if err != nil {
		t.Fatalf("NewBlockStore failed: %v", err)
	}
	bot := &Bot{
		worldState: &world.WorldState{},
		blockStore: blockStore,
	}

	payload := buildChunkPacketPayloadWithBlockEntities(
		t,
		0,
		0,
		map[int]int32{7: 1234},
		[]chunkBlockEntityPayload{
			{
				LocalX: 5,
				LocalZ: 7,
				Y:      70,
				TypeID: 42,
				NBTTag: protocol.TagInt,
				NBTInt: 1,
			},
		},
	)
	bot.handleLevelChunkWithLight(payload)

	tileUpdate := new(bytes.Buffer)
	_ = protocol.WriteInt64(tileUpdate, packBlockPosition(5, 70, 7))
	_ = protocol.WriteVarint(tileUpdate, 7)
	_ = protocol.WriteByte(tileUpdate, protocol.TagInt)
	_ = protocol.WriteInt32(tileUpdate, 777)
	bot.handleTileEntityData(tileUpdate.Bytes())

	entity, ok := bot.blockStore.GetBlockEntity(5, 70, 7)
	if !ok {
		t.Fatalf("GetBlockEntity should return updated block entity")
	}
	if entity.TypeID != 42 {
		t.Fatalf("TypeID after tile update = %d, want 42", entity.TypeID)
	}
	if !entity.HasAction || entity.Action != 7 {
		t.Fatalf("tile action = (has:%v action:%d), want (true,7)", entity.HasAction, entity.Action)
	}

	nbtNode, ok := entity.NBTData.(*protocol.NBTNode)
	if !ok || nbtNode == nil {
		t.Fatalf("NBTData should be *protocol.NBTNode, got %+v", entity.NBTData)
	}
	if nbtNode.Type != protocol.TagInt {
		t.Fatalf("NBTData.Type = %d, want TagInt", nbtNode.Type)
	}
	if v, ok := nbtNode.Value.(int32); !ok || v != 777 {
		t.Fatalf("unexpected NBT value after tile update: %+v", nbtNode.Value)
	}

	unloadPayload := new(bytes.Buffer)
	_ = protocol.WriteInt32(unloadPayload, 0) // chunkZ first
	_ = protocol.WriteInt32(unloadPayload, 0) // chunkX
	bot.handleUnloadChunk(unloadPayload.Bytes())
	if _, ok := bot.blockStore.GetBlockEntity(5, 70, 7); ok {
		t.Fatalf("GetBlockEntity should return false after unload")
	}
}

func TestHandleMultiBlockChangeUpdatesLoadedChunk(t *testing.T) {
	blockStore, err := world.NewBlockStore()
	if err != nil {
		t.Fatalf("NewBlockStore failed: %v", err)
	}
	bot := &Bot{
		worldState: &world.WorldState{},
		blockStore: blockStore,
	}

	payload := buildChunkPacketPayload(t, 0, 0, map[int]int32{
		7: 1234,
	})
	bot.handleLevelChunkWithLight(payload)

	multi := new(bytes.Buffer)
	_ = protocol.WriteInt64(multi, packChunkSectionPosition(0, 3, 0))
	_ = protocol.WriteVarint(multi, 1)
	// y=63 -> sectionY=3, localY=15
	_ = protocol.WriteVarint(multi, packMultiBlockRecord(1, 1, 15, 2))
	bot.handleMultiBlockChange(multi.Bytes())

	after, ok := bot.GetBlockState(1, 63, 2)
	if !ok {
		t.Fatalf("GetBlockState should return ok=true after multi block change")
	}
	if after != 1 {
		t.Fatalf("after block state = %d, want 1", after)
	}
}

func TestHandleBlockActionStoresRecentAction(t *testing.T) {
	blockStore, err := world.NewBlockStore()
	if err != nil {
		t.Fatalf("NewBlockStore failed: %v", err)
	}
	bot := &Bot{
		worldState: &world.WorldState{},
		blockStore: blockStore,
	}

	payload := buildChunkPacketPayload(t, 0, 0, map[int]int32{7: 1234})
	bot.handleLevelChunkWithLight(payload)

	actionPayload := new(bytes.Buffer)
	_ = protocol.WriteInt64(actionPayload, packBlockPosition(5, 70, 7))
	_ = protocol.WriteByte(actionPayload, 1)
	_ = protocol.WriteByte(actionPayload, 2)
	_ = protocol.WriteVarint(actionPayload, 33)
	bot.handleBlockAction(actionPayload.Bytes())

	action, ok := bot.blockStore.GetLastBlockAction(5, 70, 7)
	if !ok {
		t.Fatalf("GetLastBlockAction should return recorded action")
	}
	if action.Byte1 != 1 || action.Byte2 != 2 || action.BlockID != 33 {
		t.Fatalf("unexpected action payload: %+v", action)
	}
}

func TestHandlePlayLoginAndRespawnUpdatesDimensionAndClearsChunks(t *testing.T) {
	blockStore, err := world.NewBlockStore()
	if err != nil {
		t.Fatalf("NewBlockStore failed: %v", err)
	}

	bot := &Bot{
		worldState: &world.WorldState{},
		blockStore: blockStore,
	}

	chunkPayload := buildChunkPacketPayload(t, 0, 0, map[int]int32{7: 1234})
	bot.handleLevelChunkWithLight(chunkPayload)
	if bot.blockStore.LoadedChunkCount() != 1 {
		t.Fatalf("LoadedChunkCount before respawn = %d, want 1", bot.blockStore.LoadedChunkCount())
	}
	if _, ok := bot.GetBlockState(1, 63, 2); !ok {
		t.Fatalf("expected existing block before respawn")
	}

	loginPayload := buildPlayLoginPayloadForTest(world.DimensionOverworld, 10)
	bot.handlePlayLogin(loginPayload)
	viewPayload := buildUpdateViewPositionPayloadForTest(-2, 3)
	bot.handleUpdateViewPosition(viewPayload)

	snap := bot.GetState()
	if snap.DimensionName != world.DimensionOverworld {
		t.Fatalf("DimensionName after login = %q, want %q", snap.DimensionName, world.DimensionOverworld)
	}
	if snap.SimulationDistance != 10 {
		t.Fatalf("SimulationDistance after login = %d, want 10", snap.SimulationDistance)
	}
	if snap.ViewCenterChunkX != -2 || snap.ViewCenterChunkZ != 3 {
		t.Fatalf(
			"ViewCenter after update = (%d,%d), want (-2,3)",
			snap.ViewCenterChunkX,
			snap.ViewCenterChunkZ,
		)
	}

	respawnPayload := buildRespawnPayloadForTest(world.DimensionNether)
	bot.handleRespawn(respawnPayload)

	snap = bot.GetState()
	if snap.DimensionName != world.DimensionNether {
		t.Fatalf("DimensionName after respawn = %q, want %q", snap.DimensionName, world.DimensionNether)
	}
	if snap.SimulationDistance != 10 {
		t.Fatalf("SimulationDistance after respawn = %d, want 10", snap.SimulationDistance)
	}
	if bot.blockStore.LoadedChunkCount() != 0 {
		t.Fatalf("LoadedChunkCount after respawn = %d, want 0", bot.blockStore.LoadedChunkCount())
	}
	if _, ok := bot.GetBlockState(1, 63, 2); ok {
		t.Fatalf("old chunk block should not be queryable after respawn clear")
	}
}

func TestHandleChunkBatchFinishedSendsAck(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	bot := &Bot{
		conn:      client,
		connState: protocol.NewConnState(),
	}

	errCh := make(chan error, 1)
	go func() {
		packet, err := protocol.ReadPacket(server, -1)
		if err != nil {
			errCh <- err
			return
		}
		if packet.ID != protocol.C2SChunkBatchReceived {
			errCh <- errors.New("unexpected packet id")
			return
		}

		r := bytes.NewReader(packet.Payload)
		chunksPerTick, err := protocol.ReadFloat(r)
		if err != nil {
			errCh <- err
			return
		}
		if chunksPerTick != 20.0 {
			errCh <- errors.New("unexpected chunksPerTick value")
			return
		}
		errCh <- nil
	}()

	payload := new(bytes.Buffer)
	_ = protocol.WriteVarint(payload, 128)
	bot.handleChunkBatchFinished(payload.Bytes())

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("chunk batch ack validation failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for chunk batch ack packet")
	}
}

func packBlockPosition(x, y, z int32) int64 {
	ux := uint64(int64(x) & 0x3FFFFFF)
	uy := uint64(int64(y) & 0xFFF)
	uz := uint64(int64(z) & 0x3FFFFFF)
	return int64((ux << 38) | (uz << 12) | uy)
}

func packChunkSectionPosition(chunkX, chunkY, chunkZ int32) int64 {
	ux := uint64(int64(chunkX) & 0x3FFFFF)
	uy := uint64(int64(chunkY) & 0xFFFFF)
	uz := uint64(int64(chunkZ) & 0x3FFFFF)
	return int64((ux << 42) | (uz << 20) | uy)
}

func packMultiBlockRecord(stateID, localX, localY, localZ int32) int32 {
	local := ((localX & 0x0F) << 8) | ((localZ & 0x0F) << 4) | (localY & 0x0F)
	return (stateID << 12) | local
}

func buildPlayLoginPayloadForTest(dimensionName string, simulationDistance int32) []byte {
	buf := new(bytes.Buffer)
	_ = protocol.WriteInt32(buf, 1) // entityId
	_ = protocol.WriteBool(buf, false)
	_ = protocol.WriteVarint(buf, 1)
	_ = protocol.WriteString(buf, dimensionName)
	_ = protocol.WriteVarint(buf, 20) // maxPlayers
	_ = protocol.WriteVarint(buf, 10) // viewDistance
	_ = protocol.WriteVarint(buf, simulationDistance)
	_ = protocol.WriteBool(buf, false) // reducedDebugInfo
	_ = protocol.WriteBool(buf, true)  // enableRespawnScreen
	_ = protocol.WriteBool(buf, false) // doLimitedCrafting
	writeSpawnInfoForBotTest(buf, dimensionName)
	_ = protocol.WriteBool(buf, false) // enforcesSecureChat
	return buf.Bytes()
}

func buildRespawnPayloadForTest(dimensionName string) []byte {
	buf := new(bytes.Buffer)
	writeSpawnInfoForBotTest(buf, dimensionName)
	_ = protocol.WriteByte(buf, 0) // copyMetadata
	return buf.Bytes()
}

func buildUpdateViewPositionPayloadForTest(chunkX, chunkZ int32) []byte {
	buf := new(bytes.Buffer)
	_ = protocol.WriteVarint(buf, chunkX)
	_ = protocol.WriteVarint(buf, chunkZ)
	return buf.Bytes()
}

func writeSpawnInfoForBotTest(buf *bytes.Buffer, dimensionName string) {
	_ = protocol.WriteVarint(buf, dimensionIDForTest(dimensionName))
	_ = protocol.WriteString(buf, dimensionName)
	_ = protocol.WriteInt64(buf, 12345) // hashedSeed
	_ = protocol.WriteByte(buf, 0)      // gamemode
	_ = protocol.WriteByte(buf, 255)    // previousGamemode
	_ = protocol.WriteBool(buf, false)  // isDebug
	_ = protocol.WriteBool(buf, false)  // isFlat
	_ = protocol.WriteBool(buf, false)  // death absent
	_ = protocol.WriteVarint(buf, 0)    // portalCooldown
	_ = protocol.WriteVarint(buf, 63)   // seaLevel
}

func dimensionIDForTest(dimensionName string) int32 {
	switch dimensionName {
	case world.DimensionOverworld:
		return 0
	case world.DimensionNether:
		return 1
	case world.DimensionEnd:
		return 2
	default:
		return 0
	}
}
