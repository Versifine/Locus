package protocol

import (
	"fmt"
	"io"
)

type UpdateViewPosition struct {
	ChunkX int32
	ChunkZ int32
}

func ParseUpdateViewPosition(r io.Reader) (*UpdateViewPosition, error) {
	chunkX, err := ReadVarint(r)
	if err != nil {
		return nil, fmt.Errorf("read chunkX: %w", err)
	}

	chunkZ, err := ReadVarint(r)
	if err != nil {
		return nil, fmt.Errorf("read chunkZ: %w", err)
	}

	return &UpdateViewPosition{
		ChunkX: chunkX,
		ChunkZ: chunkZ,
	}, nil
}
