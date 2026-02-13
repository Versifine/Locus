package protocol

import (
	"bytes"
	"fmt"
	"io"
)

type ChunkBatchStart struct{}

type ChunkBatchFinished struct {
	BatchSize int32
}

func ParseChunkBatchStart(_ io.Reader) (*ChunkBatchStart, error) {
	return &ChunkBatchStart{}, nil
}

func ParseChunkBatchFinished(r io.Reader) (*ChunkBatchFinished, error) {
	batchSize, err := ReadVarint(r)
	if err != nil {
		return nil, fmt.Errorf("read batch size: %w", err)
	}
	if batchSize < 0 {
		return nil, fmt.Errorf("invalid batch size: %d", batchSize)
	}

	return &ChunkBatchFinished{BatchSize: batchSize}, nil
}

func CreateChunkBatchReceivedPacket(chunksPerTick float32) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteFloat(buf, chunksPerTick)
	return &Packet{
		ID:      C2SChunkBatchReceived,
		Payload: buf.Bytes(),
	}
}
