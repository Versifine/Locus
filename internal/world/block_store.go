package world

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const (
	ChunkMinY          = -64
	ChunkMaxY          = 319
	ChunkSectionCount  = 24
	ChunkSectionHeight = 16
	BlocksPerSection   = 16 * 16 * 16
)

type ChunkPos struct {
	X int32
	Z int32
}

type ChunkSection struct {
	BlockStates []int32
}

type Chunk struct {
	Sections []ChunkSection
}

type BlockStore struct {
	mu                 sync.RWMutex
	chunks             map[ChunkPos]*Chunk
	solidByStateID     []bool
	blockNameByStateID []string
}

type blockDefinition struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	MinStateID  int32  `json:"minStateId"`
	MaxStateID  int32  `json:"maxStateId"`
	BoundingBox string `json:"boundingBox"`
}

func NewBlockStore() (*BlockStore, error) {
	return NewBlockStoreFromBlocksJSON(defaultBlocksJSONPath())
}

func NewBlockStoreFromBlocksJSON(blocksJSONPath string) (*BlockStore, error) {
	solidByStateID, blockNameByStateID, err := LoadStateMetadataFromBlocksJSON(blocksJSONPath)
	if err != nil {
		return nil, err
	}
	return &BlockStore{
		chunks:             make(map[ChunkPos]*Chunk),
		solidByStateID:     solidByStateID,
		blockNameByStateID: blockNameByStateID,
	}, nil
}

func LoadStateSolidityFromBlocksJSON(blocksJSONPath string) ([]bool, error) {
	solidByStateID, _, err := LoadStateMetadataFromBlocksJSON(blocksJSONPath)
	return solidByStateID, err
}

func LoadStateMetadataFromBlocksJSON(blocksJSONPath string) ([]bool, []string, error) {
	if blocksJSONPath == "" {
		return nil, nil, fmt.Errorf("blocks.json path is empty")
	}

	data, err := os.ReadFile(blocksJSONPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read blocks.json: %w", err)
	}

	var blocks []blockDefinition
	if err := json.Unmarshal(data, &blocks); err != nil {
		return nil, nil, fmt.Errorf("parse blocks.json: %w", err)
	}
	if len(blocks) == 0 {
		return nil, nil, fmt.Errorf("blocks.json has no block definitions")
	}

	maxStateID := int32(-1)
	for _, block := range blocks {
		if block.MinStateID < 0 || block.MaxStateID < block.MinStateID {
			return nil, nil, fmt.Errorf(
				"invalid state id range in blocks.json: min=%d max=%d",
				block.MinStateID,
				block.MaxStateID,
			)
		}
		if block.MaxStateID > maxStateID {
			maxStateID = block.MaxStateID
		}
	}

	solidByStateID := make([]bool, int(maxStateID)+1)
	blockNameByStateID := make([]string, int(maxStateID)+1)
	for _, block := range blocks {
		isSolid := block.BoundingBox == "block"
		blockName := block.DisplayName
		if blockName == "" {
			blockName = block.Name
		}
		for id := int(block.MinStateID); id <= int(block.MaxStateID); id++ {
			solidByStateID[id] = isSolid
			if blockNameByStateID[id] == "" {
				blockNameByStateID[id] = blockName
			}
		}
	}
	return solidByStateID, blockNameByStateID, nil
}

func (bs *BlockStore) StoreChunk(chunkX, chunkZ int32, sections []ChunkSection) error {
	if len(sections) != ChunkSectionCount {
		return fmt.Errorf("invalid section count: got %d, want %d", len(sections), ChunkSectionCount)
	}

	chunk := &Chunk{
		Sections: make([]ChunkSection, ChunkSectionCount),
	}

	for i := range sections {
		if len(sections[i].BlockStates) != BlocksPerSection {
			return fmt.Errorf(
				"invalid section %d block state count: got %d, want %d",
				i,
				len(sections[i].BlockStates),
				BlocksPerSection,
			)
		}

		copied := make([]int32, BlocksPerSection)
		copy(copied, sections[i].BlockStates)
		chunk.Sections[i] = ChunkSection{BlockStates: copied}
	}

	bs.mu.Lock()
	defer bs.mu.Unlock()
	if bs.chunks == nil {
		bs.chunks = make(map[ChunkPos]*Chunk)
	}
	bs.chunks[ChunkPos{X: chunkX, Z: chunkZ}] = chunk
	return nil
}

func (bs *BlockStore) UnloadChunk(chunkX, chunkZ int32) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	delete(bs.chunks, ChunkPos{X: chunkX, Z: chunkZ})
}

func (bs *BlockStore) SetBlockState(x, y, z int, stateID int32) bool {
	if y < ChunkMinY || y > ChunkMaxY {
		return false
	}

	chunkX := floorDiv16(x)
	chunkZ := floorDiv16(z)
	localX := floorMod16(x)
	localZ := floorMod16(z)
	sectionIndex := (y - ChunkMinY) / ChunkSectionHeight
	localY := (y - ChunkMinY) % ChunkSectionHeight
	blockIndex := localY*16*16 + localZ*16 + localX

	bs.mu.Lock()
	defer bs.mu.Unlock()

	chunk, ok := bs.chunks[ChunkPos{X: int32(chunkX), Z: int32(chunkZ)}]
	if !ok {
		return false
	}
	if sectionIndex < 0 || sectionIndex >= len(chunk.Sections) {
		return false
	}
	if blockIndex < 0 || blockIndex >= len(chunk.Sections[sectionIndex].BlockStates) {
		return false
	}

	chunk.Sections[sectionIndex].BlockStates[blockIndex] = stateID
	return true
}

func (bs *BlockStore) IsLoaded(chunkX, chunkZ int32) bool {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	_, ok := bs.chunks[ChunkPos{X: chunkX, Z: chunkZ}]
	return ok
}

func (bs *BlockStore) LoadedChunkCount() int {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return len(bs.chunks)
}

func (bs *BlockStore) GetBlockState(x, y, z int) (int32, bool) {
	if y < ChunkMinY || y > ChunkMaxY {
		return 0, false
	}

	chunkX := floorDiv16(x)
	chunkZ := floorDiv16(z)
	localX := floorMod16(x)
	localZ := floorMod16(z)

	sectionIndex := (y - ChunkMinY) / ChunkSectionHeight
	localY := (y - ChunkMinY) % ChunkSectionHeight
	blockIndex := localY*16*16 + localZ*16 + localX

	bs.mu.RLock()
	defer bs.mu.RUnlock()

	chunk, ok := bs.chunks[ChunkPos{X: int32(chunkX), Z: int32(chunkZ)}]
	if !ok {
		return 0, false
	}
	if sectionIndex < 0 || sectionIndex >= len(chunk.Sections) {
		return 0, false
	}
	section := chunk.Sections[sectionIndex]
	if blockIndex < 0 || blockIndex >= len(section.BlockStates) {
		return 0, false
	}

	return section.BlockStates[blockIndex], true
}

func (bs *BlockStore) IsSolid(x, y, z int) bool {
	stateID, ok := bs.GetBlockState(x, y, z)
	if !ok || stateID < 0 {
		return false
	}
	if int(stateID) >= len(bs.solidByStateID) {
		return false
	}
	return bs.solidByStateID[stateID]
}

func (bs *BlockStore) GetBlockNameByStateID(stateID int32) (string, bool) {
	if stateID < 0 {
		return "", false
	}

	bs.mu.RLock()
	defer bs.mu.RUnlock()

	if int(stateID) >= len(bs.blockNameByStateID) {
		return "", false
	}
	name := bs.blockNameByStateID[stateID]
	if name == "" {
		return "", false
	}
	return name, true
}

func defaultBlocksJSONPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("1.21.11", "blocks.json")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "1.21.11", "blocks.json"))
}

func floorDiv16(v int) int {
	q := v / 16
	if v < 0 && v%16 != 0 {
		q--
	}
	return q
}

func floorMod16(v int) int {
	m := v % 16
	if m < 0 {
		m += 16
	}
	return m
}
