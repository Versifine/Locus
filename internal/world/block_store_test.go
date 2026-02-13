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
