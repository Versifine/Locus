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

func buildChunkPacketPayload(t *testing.T, chunkX, chunkZ int32, sectionStates map[int]int32) []byte {
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
	_ = protocol.WriteVarint(payload, 0) // block entities count
	for i := 0; i < 6; i++ {             // light data arrays
		_ = protocol.WriteVarint(payload, 0)
	}

	return payload.Bytes()
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
