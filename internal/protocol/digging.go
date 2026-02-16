package protocol

import (
	"bytes"
	"fmt"
	"io"
)

const (
	BlockDigStatusStarted   int32 = 0
	BlockDigStatusCancelled int32 = 1
	BlockDigStatusFinished  int32 = 2
)

type BlockPos struct {
	X int32
	Y int32
	Z int32
}

type AcknowledgePlayerDigging struct {
	SequenceID int32
}

func ParseAcknowledgePlayerDigging(r io.Reader) (*AcknowledgePlayerDigging, error) {
	sequenceID, err := ReadVarint(r)
	if err != nil {
		return nil, fmt.Errorf("read sequence id: %w", err)
	}
	if sequenceID < 0 {
		return nil, fmt.Errorf("invalid sequence id: %d", sequenceID)
	}

	return &AcknowledgePlayerDigging{SequenceID: sequenceID}, nil
}

func CreateBlockDigPacket(status int32, location BlockPos, face int8, sequence int32) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, status)
	_ = WriteInt64(buf, encodeBlockPosition(location.X, location.Y, location.Z))
	_ = WriteByte(buf, byte(face))
	_ = WriteVarint(buf, sequence)
	return &Packet{
		ID:      C2SBlockDig,
		Payload: buf.Bytes(),
	}
}

func encodeBlockPosition(x, y, z int32) int64 {
	ux := uint64(int64(x) & 0x3FFFFFF)
	uy := uint64(int64(y) & 0xFFF)
	uz := uint64(int64(z) & 0x3FFFFFF)
	return int64((ux << 38) | (uz << 12) | uy)
}
