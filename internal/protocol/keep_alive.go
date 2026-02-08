package protocol

import "bytes"

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
