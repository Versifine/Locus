package protocol

import (
	"bytes"
	"fmt"
	"io"
)

const (
	ChunkSectionCount     = 24
	BlockStatesPerSection = 16 * 16 * 16
	BiomesPerSection      = 4 * 4 * 4
)

// ChunkSection contains expanded block-state IDs for a 16x16x16 section.
type ChunkSection struct {
	BlockCount  int16
	BlockStates []int32
}

// LevelChunkWithLight is the S2C map_chunk packet payload (protocol 774).
type LevelChunkWithLight struct {
	ChunkX           int32
	ChunkZ           int32
	Heightmaps       *NBTNode
	ChunkData        []byte
	Sections         []ChunkSection
	BlockEntityCount int32
}

func ParseLevelChunkWithLight(r io.Reader) (*LevelChunkWithLight, error) {
	chunkX, err := ReadInt32(r)
	if err != nil {
		return nil, err
	}
	chunkZ, err := ReadInt32(r)
	if err != nil {
		return nil, err
	}

	heightmaps, err := ReadAnonymousNBT(r)
	if err != nil {
		return nil, err
	}

	chunkData, err := readVarIntByteArray(r)
	if err != nil {
		return nil, err
	}

	sections, err := ParseChunkSections(chunkData, ChunkSectionCount)
	if err != nil {
		return nil, err
	}

	blockEntityCount, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	if blockEntityCount < 0 {
		return nil, fmt.Errorf("invalid block entity count: %d", blockEntityCount)
	}
	for i := int32(0); i < blockEntityCount; i++ {
		if err := skipChunkBlockEntity(r); err != nil {
			return nil, fmt.Errorf("failed to parse block entity %d: %w", i, err)
		}
	}

	if err := skipLightData(r); err != nil {
		return nil, err
	}

	return &LevelChunkWithLight{
		ChunkX:           chunkX,
		ChunkZ:           chunkZ,
		Heightmaps:       heightmaps,
		ChunkData:        chunkData,
		Sections:         sections,
		BlockEntityCount: blockEntityCount,
	}, nil
}

func ParseChunkSections(chunkData []byte, sectionCount int) ([]ChunkSection, error) {
	if sectionCount <= 0 {
		return nil, fmt.Errorf("invalid section count: %d", sectionCount)
	}

	reader := bytes.NewReader(chunkData)
	sections := make([]ChunkSection, sectionCount)
	for i := 0; i < sectionCount; i++ {
		blockCount, err := ReadInt16(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read section %d block count: %w", i, err)
		}

		blockStates, err := ParsePalettedContainer(reader, BlockStatesPerSection)
		if err != nil {
			return nil, fmt.Errorf("failed to read section %d block states: %w", i, err)
		}

		if _, err := ParsePalettedContainer(reader, BiomesPerSection); err != nil {
			return nil, fmt.Errorf("failed to read section %d biomes: %w", i, err)
		}

		sections[i] = ChunkSection{
			BlockCount:  blockCount,
			BlockStates: blockStates,
		}
	}

	if reader.Len() != 0 {
		return nil, fmt.Errorf("chunk data has %d unread bytes", reader.Len())
	}
	return sections, nil
}

func readVarIntByteArray(r io.Reader) ([]byte, error) {
	length, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("invalid byte array length: %d", length)
	}
	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func skipChunkBlockEntity(r io.Reader) error {
	if _, err := ReadByte(r); err != nil { // packed x/z nibble
		return err
	}
	if _, err := ReadInt16(r); err != nil {
		return err
	}
	if _, err := ReadVarint(r); err != nil {
		return err
	}
	_, err := ReadAnonymousNBT(r)
	return err
}

func skipLightData(r io.Reader) error {
	// skyLightMask, blockLightMask, emptySkyLightMask, emptyBlockLightMask
	for i := 0; i < 4; i++ {
		if err := skipInt64Array(r); err != nil {
			return err
		}
	}
	// skyLight, blockLight
	for i := 0; i < 2; i++ {
		if err := skipByteArrayArray(r); err != nil {
			return err
		}
	}
	return nil
}

func skipInt64Array(r io.Reader) error {
	count, err := ReadVarint(r)
	if err != nil {
		return err
	}
	if count < 0 {
		return fmt.Errorf("invalid int64 array length: %d", count)
	}
	for i := int32(0); i < count; i++ {
		if _, err := ReadInt64(r); err != nil {
			return err
		}
	}
	return nil
}

func skipByteArrayArray(r io.Reader) error {
	count, err := ReadVarint(r)
	if err != nil {
		return err
	}
	if count < 0 {
		return fmt.Errorf("invalid byte-array array length: %d", count)
	}
	for i := int32(0); i < count; i++ {
		if _, err := readVarIntByteArray(r); err != nil {
			return err
		}
	}
	return nil
}
