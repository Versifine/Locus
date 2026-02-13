package protocol

import (
	"fmt"
	"io"
)

type TileEntityData struct {
	X       int32
	Y       int32
	Z       int32
	Action  int32
	NBTData *NBTNode
}

type BlockAction struct {
	X       int32
	Y       int32
	Z       int32
	Byte1   byte
	Byte2   byte
	BlockID int32
}

func ParseTileEntityData(r io.Reader) (*TileEntityData, error) {
	x, y, z, err := readPackedBlockPosition(r)
	if err != nil {
		return nil, fmt.Errorf("read tile entity position: %w", err)
	}

	action, err := ReadVarint(r)
	if err != nil {
		return nil, fmt.Errorf("read tile entity action: %w", err)
	}

	nbtData, err := ReadAnonymousNBT(r)
	if err != nil {
		return nil, fmt.Errorf("read tile entity NBT: %w", err)
	}
	if nbtData != nil && nbtData.Type == TagEnd {
		nbtData = nil
	}

	return &TileEntityData{
		X:       x,
		Y:       y,
		Z:       z,
		Action:  action,
		NBTData: nbtData,
	}, nil
}

func ParseBlockAction(r io.Reader) (*BlockAction, error) {
	x, y, z, err := readPackedBlockPosition(r)
	if err != nil {
		return nil, fmt.Errorf("read block action position: %w", err)
	}

	byte1, err := ReadByte(r)
	if err != nil {
		return nil, fmt.Errorf("read block action byte1: %w", err)
	}
	byte2, err := ReadByte(r)
	if err != nil {
		return nil, fmt.Errorf("read block action byte2: %w", err)
	}

	blockID, err := ReadVarint(r)
	if err != nil {
		return nil, fmt.Errorf("read block action block id: %w", err)
	}
	if blockID < 0 {
		return nil, fmt.Errorf("invalid block id: %d", blockID)
	}

	return &BlockAction{
		X:       x,
		Y:       y,
		Z:       z,
		Byte1:   byte1,
		Byte2:   byte2,
		BlockID: blockID,
	}, nil
}
