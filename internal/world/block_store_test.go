package world

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStateSolidityFromBlocksJSON(t *testing.T) {
	tmpDir := t.TempDir()
	blocksJSONPath := filepath.Join(tmpDir, "blocks.json")

	content := `[
  {"minStateId":0,"maxStateId":0,"boundingBox":"empty"},
  {"minStateId":1,"maxStateId":3,"boundingBox":"block"},
  {"minStateId":4,"maxStateId":5,"boundingBox":"empty"}
]`

	if err := os.WriteFile(blocksJSONPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp blocks.json failed: %v", err)
	}

	solidByStateID, err := LoadStateSolidityFromBlocksJSON(blocksJSONPath)
	if err != nil {
		t.Fatalf("LoadStateSolidityFromBlocksJSON failed: %v", err)
	}

	if len(solidByStateID) != 6 {
		t.Fatalf("len(solidByStateID) = %d, want 6", len(solidByStateID))
	}

	if solidByStateID[0] {
		t.Fatalf("state 0 should be non-solid")
	}
	if !solidByStateID[1] || !solidByStateID[2] || !solidByStateID[3] {
		t.Fatalf("states 1~3 should be solid")
	}
	if solidByStateID[4] || solidByStateID[5] {
		t.Fatalf("states 4~5 should be non-solid")
	}
}

func TestLoadStateMetadataFromBlocksJSON(t *testing.T) {
	tmpDir := t.TempDir()
	blocksJSONPath := filepath.Join(tmpDir, "blocks.json")

	content := `[
  {"name":"air","displayName":"Air","minStateId":0,"maxStateId":0,"boundingBox":"empty"},
  {"name":"stone","displayName":"Stone","minStateId":1,"maxStateId":3,"boundingBox":"block"},
  {"name":"short_grass","displayName":"Short Grass","minStateId":4,"maxStateId":5,"boundingBox":"empty"}
]`
	if err := os.WriteFile(blocksJSONPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp blocks.json failed: %v", err)
	}

	solidByStateID, nameByStateID, err := LoadStateMetadataFromBlocksJSON(blocksJSONPath)
	if err != nil {
		t.Fatalf("LoadStateMetadataFromBlocksJSON failed: %v", err)
	}
	if len(solidByStateID) != 6 {
		t.Fatalf("len(solidByStateID) = %d, want 6", len(solidByStateID))
	}
	if len(nameByStateID) != 6 {
		t.Fatalf("len(nameByStateID) = %d, want 6", len(nameByStateID))
	}
	if nameByStateID[0] != "Air" {
		t.Fatalf("nameByStateID[0] = %q, want %q", nameByStateID[0], "Air")
	}
	if nameByStateID[2] != "Stone" {
		t.Fatalf("nameByStateID[2] = %q, want %q", nameByStateID[2], "Stone")
	}
	if nameByStateID[5] != "Short Grass" {
		t.Fatalf("nameByStateID[5] = %q, want %q", nameByStateID[5], "Short Grass")
	}
}

func TestBlockStoreStoreGetAndUnloadChunk(t *testing.T) {
	bs := &BlockStore{
		chunks:         make(map[ChunkPos]*Chunk),
		solidByStateID: []bool{false, true},
	}

	sections := makeFilledSections(0)

	// Global (2,70,3) belongs to chunk (0,0), section index 8, localY 6.
	sectionIndex := (70 - ChunkMinY) / ChunkSectionHeight
	localY := (70 - ChunkMinY) % ChunkSectionHeight
	index := localY*16*16 + 3*16 + 2
	sections[sectionIndex].BlockStates[index] = 1

	if err := bs.StoreChunk(0, 0, sections); err != nil {
		t.Fatalf("StoreChunk failed: %v", err)
	}

	if !bs.IsLoaded(0, 0) {
		t.Fatalf("chunk (0,0) should be loaded")
	}
	if bs.LoadedChunkCount() != 1 {
		t.Fatalf("LoadedChunkCount = %d, want 1", bs.LoadedChunkCount())
	}

	state, ok := bs.GetBlockState(2, 70, 3)
	if !ok {
		t.Fatalf("GetBlockState returned not loaded for existing chunk")
	}
	if state != 1 {
		t.Fatalf("GetBlockState = %d, want 1", state)
	}
	if !bs.IsSolid(2, 70, 3) {
		t.Fatalf("IsSolid should be true for state 1")
	}

	bs.UnloadChunk(0, 0)
	if bs.IsLoaded(0, 0) {
		t.Fatalf("chunk (0,0) should be unloaded")
	}
	if bs.LoadedChunkCount() != 0 {
		t.Fatalf("LoadedChunkCount = %d, want 0", bs.LoadedChunkCount())
	}
	if _, ok := bs.GetBlockState(2, 70, 3); ok {
		t.Fatalf("GetBlockState should return false after unload")
	}
}

func TestBlockStoreNegativeCoordinatesAndSolidEdgeCases(t *testing.T) {
	bs := &BlockStore{
		chunks:         make(map[ChunkPos]*Chunk),
		solidByStateID: []bool{false, true},
	}

	sections := makeFilledSections(0)
	// Global (-1,-64,-1) belongs to chunk (-1,-1), local (15,0,15).
	index := 0*16*16 + 15*16 + 15
	sections[0].BlockStates[index] = 1
	if err := bs.StoreChunk(-1, -1, sections); err != nil {
		t.Fatalf("StoreChunk failed: %v", err)
	}

	state, ok := bs.GetBlockState(-1, -64, -1)
	if !ok || state != 1 {
		t.Fatalf("GetBlockState(-1,-64,-1) = (%d,%v), want (1,true)", state, ok)
	}
	if !bs.IsSolid(-1, -64, -1) {
		t.Fatalf("IsSolid should be true at (-1,-64,-1)")
	}

	// Y out of world bounds.
	if bs.IsSolid(-1, ChunkMinY-1, -1) {
		t.Fatalf("IsSolid should be false when Y is below minimum")
	}
	if bs.IsSolid(-1, ChunkMaxY+1, -1) {
		t.Fatalf("IsSolid should be false when Y is above maximum")
	}

	// Unloaded chunk should return false.
	if bs.IsSolid(0, 64, 0) {
		t.Fatalf("IsSolid should be false for unloaded chunk")
	}
}

func TestBlockStoreStoreChunkValidation(t *testing.T) {
	bs := &BlockStore{
		chunks:         make(map[ChunkPos]*Chunk),
		solidByStateID: []bool{false},
	}

	if err := bs.StoreChunk(0, 0, make([]ChunkSection, ChunkSectionCount-1)); err == nil {
		t.Fatalf("expected error when section count is invalid")
	}

	sections := makeFilledSections(0)
	sections[3].BlockStates = make([]int32, BlocksPerSection-1)
	if err := bs.StoreChunk(0, 0, sections); err == nil {
		t.Fatalf("expected error when section block-state count is invalid")
	}
}

func TestGetBlockNameByStateID(t *testing.T) {
	bs := &BlockStore{
		chunks:             make(map[ChunkPos]*Chunk),
		solidByStateID:     []bool{false, true},
		blockNameByStateID: []string{"Air", "Stone"},
	}

	name, ok := bs.GetBlockNameByStateID(1)
	if !ok {
		t.Fatalf("GetBlockNameByStateID(1) should return ok=true")
	}
	if name != "Stone" {
		t.Fatalf("GetBlockNameByStateID(1) = %q, want %q", name, "Stone")
	}

	if _, ok := bs.GetBlockNameByStateID(99); ok {
		t.Fatalf("GetBlockNameByStateID(99) should return ok=false")
	}
}

func TestSetBlockState(t *testing.T) {
	bs := &BlockStore{
		chunks:         make(map[ChunkPos]*Chunk),
		solidByStateID: []bool{false, true},
	}
	sections := makeFilledSections(0)
	if err := bs.StoreChunk(0, 0, sections); err != nil {
		t.Fatalf("StoreChunk failed: %v", err)
	}

	if ok := bs.SetBlockState(2, 70, 3, 1); !ok {
		t.Fatalf("SetBlockState should return true for loaded block")
	}
	got, ok := bs.GetBlockState(2, 70, 3)
	if !ok {
		t.Fatalf("GetBlockState should return ok=true after SetBlockState")
	}
	if got != 1 {
		t.Fatalf("GetBlockState = %d, want 1", got)
	}
}

func TestSetBlockStateUnloadedChunk(t *testing.T) {
	bs := &BlockStore{
		chunks:         make(map[ChunkPos]*Chunk),
		solidByStateID: []bool{false, true},
	}
	if ok := bs.SetBlockState(2, 70, 3, 1); ok {
		t.Fatalf("SetBlockState should return false for unloaded chunk")
	}
}

func TestClearRemovesAllChunks(t *testing.T) {
	bs := &BlockStore{
		chunks:         make(map[ChunkPos]*Chunk),
		solidByStateID: []bool{false, true},
	}

	sections := makeFilledSections(0)
	if err := bs.StoreChunk(0, 0, sections); err != nil {
		t.Fatalf("StoreChunk(0,0) failed: %v", err)
	}
	if err := bs.StoreChunk(1, -1, sections); err != nil {
		t.Fatalf("StoreChunk(1,-1) failed: %v", err)
	}
	if bs.LoadedChunkCount() != 2 {
		t.Fatalf("LoadedChunkCount = %d, want 2", bs.LoadedChunkCount())
	}

	bs.Clear()
	if bs.LoadedChunkCount() != 0 {
		t.Fatalf("LoadedChunkCount after Clear = %d, want 0", bs.LoadedChunkCount())
	}
	if bs.IsLoaded(0, 0) || bs.IsLoaded(1, -1) {
		t.Fatalf("chunks should be unloaded after Clear")
	}
}

func TestBlockStoreBlockEntityLifecycle(t *testing.T) {
	bs := &BlockStore{
		chunks:         make(map[ChunkPos]*Chunk),
		solidByStateID: []bool{false},
	}

	sections := makeFilledSections(0)
	err := bs.StoreChunkWithBlockEntities(0, 0, sections, []BlockEntity{
		{
			X:      2,
			Y:      70,
			Z:      3,
			TypeID: 10,
			NBTData: map[string]any{
				"id": "minecraft:chest",
			},
		},
	})
	if err != nil {
		t.Fatalf("StoreChunkWithBlockEntities failed: %v", err)
	}

	entity, ok := bs.GetBlockEntity(2, 70, 3)
	if !ok {
		t.Fatalf("GetBlockEntity should return initial block entity")
	}
	if entity.TypeID != 10 {
		t.Fatalf("initial TypeID = %d, want 10", entity.TypeID)
	}
	if entity.HasAction {
		t.Fatalf("initial HasAction should be false")
	}

	if ok := bs.UpdateTileEntityData(2, 70, 3, 7, map[string]any{"custom": "value"}); !ok {
		t.Fatalf("UpdateTileEntityData should return true for loaded chunk")
	}

	entity, ok = bs.GetBlockEntity(2, 70, 3)
	if !ok {
		t.Fatalf("GetBlockEntity should return entity after tile update")
	}
	if entity.TypeID != 10 {
		t.Fatalf("TypeID after tile update = %d, want 10", entity.TypeID)
	}
	if !entity.HasAction || entity.Action != 7 {
		t.Fatalf("tile update action = (has:%v action:%d), want (true,7)", entity.HasAction, entity.Action)
	}
	nbt, ok := entity.NBTData.(map[string]any)
	if !ok || nbt["custom"] != "value" {
		t.Fatalf("tile update NBTData mismatch: %+v", entity.NBTData)
	}

	bs.UnloadChunk(0, 0)
	if _, ok := bs.GetBlockEntity(2, 70, 3); ok {
		t.Fatalf("GetBlockEntity should return false after unload")
	}
}

func TestBlockStoreRecordBlockActionLifecycle(t *testing.T) {
	bs := &BlockStore{
		chunks:         make(map[ChunkPos]*Chunk),
		solidByStateID: []bool{false},
	}

	sections := makeFilledSections(0)
	if err := bs.StoreChunk(0, 0, sections); err != nil {
		t.Fatalf("StoreChunk failed: %v", err)
	}

	if ok := bs.RecordBlockAction(2, 70, 3, 1, 2, 33); !ok {
		t.Fatalf("RecordBlockAction should return true for loaded chunk")
	}

	action, ok := bs.GetLastBlockAction(2, 70, 3)
	if !ok {
		t.Fatalf("GetLastBlockAction should return action for loaded chunk")
	}
	if action.Byte1 != 1 || action.Byte2 != 2 || action.BlockID != 33 {
		t.Fatalf("unexpected action payload: %+v", action)
	}
	if action.UpdatedAt.IsZero() {
		t.Fatalf("UpdatedAt should be set")
	}

	bs.UnloadChunk(0, 0)
	if _, ok := bs.GetLastBlockAction(2, 70, 3); ok {
		t.Fatalf("GetLastBlockAction should return false after unload")
	}
}

func makeFilledSections(fill int32) []ChunkSection {
	sections := make([]ChunkSection, ChunkSectionCount)
	for i := range sections {
		states := make([]int32, BlocksPerSection)
		for j := range states {
			states[j] = fill
		}
		sections[i] = ChunkSection{BlockStates: states}
	}
	return sections
}
