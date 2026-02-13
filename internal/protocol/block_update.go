package protocol

import (
	"fmt"
	"io"
)

type BlockChange struct {
	X       int
	Y       int
	Z       int
	StateID int32
}

type MultiBlockChange struct {
	ChunkX  int32
	ChunkY  int32
	ChunkZ  int32
	Records []MultiBlockChangeRecord
}

type MultiBlockChangeRecord struct {
	X       int
	Y       int
	Z       int
	StateID int32
}

func ParseBlockChange(r io.Reader) (*BlockChange, error) {
	x, y, z, err := readPackedBlockPosition(r)
	if err != nil {
		return nil, fmt.Errorf("read block location: %w", err)
	}

	stateID, err := ReadVarint(r)
	if err != nil {
		return nil, fmt.Errorf("read block state id: %w", err)
	}
	if stateID < 0 {
		return nil, fmt.Errorf("invalid block state id: %d", stateID)
	}

	return &BlockChange{
		X:       int(x),
		Y:       int(y),
		Z:       int(z),
		StateID: stateID,
	}, nil
}

func ParseMultiBlockChange(r io.Reader) (*MultiBlockChange, error) {
	chunkX, chunkY, chunkZ, err := readPackedChunkSectionPosition(r)
	if err != nil {
		return nil, fmt.Errorf("read chunk section coordinates: %w", err)
	}

	recordCount, err := ReadVarint(r)
	if err != nil {
		return nil, fmt.Errorf("read record count: %w", err)
	}
	if recordCount < 0 {
		return nil, fmt.Errorf("invalid record count: %d", recordCount)
	}

	records := make([]MultiBlockChangeRecord, 0, recordCount)
	for i := int32(0); i < recordCount; i++ {
		rawRecord, err := ReadVarint(r)
		if err != nil {
			return nil, fmt.Errorf("read record %d: %w", i, err)
		}
		if rawRecord < 0 {
			return nil, fmt.Errorf("invalid record %d: negative value %d", i, rawRecord)
		}

		stateID := rawRecord >> 12
		local := rawRecord & 0x0FFF

		localX := (local >> 8) & 0x0F
		localZ := (local >> 4) & 0x0F
		localY := local & 0x0F

		records = append(records, MultiBlockChangeRecord{
			X:       int(chunkX)*16 + int(localX),
			Y:       int(chunkY)*16 + int(localY),
			Z:       int(chunkZ)*16 + int(localZ),
			StateID: stateID,
		})
	}

	return &MultiBlockChange{
		ChunkX:  chunkX,
		ChunkY:  chunkY,
		ChunkZ:  chunkZ,
		Records: records,
	}, nil
}

func readPackedBlockPosition(r io.Reader) (int32, int32, int32, error) {
	raw, err := ReadInt64(r)
	if err != nil {
		return 0, 0, 0, err
	}
	v := uint64(raw)

	x := signExtendInt32(int64((v>>38)&0x3FFFFFF), 26)
	z := signExtendInt32(int64((v>>12)&0x3FFFFFF), 26)
	y := signExtendInt32(int64(v&0xFFF), 12)
	return x, y, z, nil
}

func readPackedChunkSectionPosition(r io.Reader) (int32, int32, int32, error) {
	raw, err := ReadInt64(r)
	if err != nil {
		return 0, 0, 0, err
	}
	v := uint64(raw)

	x := signExtendInt32(int64((v>>42)&0x3FFFFF), 22)
	z := signExtendInt32(int64((v>>20)&0x3FFFFF), 22)
	y := signExtendInt32(int64(v&0xFFFFF), 20)
	return x, y, z, nil
}

func signExtendInt32(value int64, bits uint) int32 {
	shift := 64 - bits
	return int32((value << shift) >> shift)
}
