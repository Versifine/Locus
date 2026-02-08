package protocol

import (
	"bytes"
	"io"
)

type KeepAlive struct {
	KeepAliveID int64
}

func CreateKeepAlivePacket(keepAliveID int64, packetID int32) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteInt64(buf, keepAliveID)
	return &Packet{
		ID:      packetID,
		Payload: buf.Bytes(),
	}
}

func ParseKeepAlive(r io.Reader) (*KeepAlive, error) {
	keepAliveID, err := ReadInt64(r)
	if err != nil {
		return nil, err
	}
	return &KeepAlive{
		KeepAliveID: keepAliveID,
	}, nil
}
