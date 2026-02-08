package protocol

import (
	"bytes"
	"io"
)

type LoginStart struct {
	Username string
	UUID     UUID
}

func ParseLoginStart(r io.Reader) (*LoginStart, error) {
	username, err := ReadString(r)
	if err != nil {
		return nil, err
	}
	var uuid UUID
	uuid, err = ReadUUID(r)
	if err != nil {
		return nil, err
	}
	return &LoginStart{
		Username: username,
		UUID:     uuid,
	}, nil
}

func CreateLoginStartPacket(username string, uuid UUID) *Packet {
	payload := make([]byte, 0)
	writer := bytes.NewBuffer(payload)
	_ = WriteString(writer, username)
	_ = WriteUUID(writer, uuid)
	return &Packet{
		ID:      C2SLoginStart,
		Payload: writer.Bytes(),
	}
}
func CreateLoginAcknowledgedPacket() *Packet {
	return &Packet{
		ID:      C2SLoginAcknowledged,
		Payload: []byte{},
	}
}
