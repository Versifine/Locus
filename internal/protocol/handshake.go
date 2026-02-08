package protocol

import (
	"bytes"
	"io"
)

type Handshake struct {
	ProtocolVersion int32
	ServerAddress   string
	ServerPort      uint16
	NextState       int32
}

func CreateHandshakePacket(protocolVersion int32, serverAddress string, serverPort uint16, nextState int32) *Packet {
	payload := make([]byte, 0)
	writer := bytes.NewBuffer(payload)
	_ = WriteVarint(writer, protocolVersion)
	_ = WriteString(writer, serverAddress)
	_ = WriteUnsignedShort(writer, serverPort)
	_ = WriteVarint(writer, nextState)
	return &Packet{
		ID:      C2SHandshake,
		Payload: writer.Bytes(),
	}
}

func ParseHandshake(r io.Reader) (*Handshake, error) {
	protocolVersion, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	serverAddress, err := ReadString(r)
	if err != nil {
		return nil, err
	}
	serverPortBytes, err := ReadUnsignedShort(r)
	if err != nil {
		return nil, err
	}
	nextState, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}

	return &Handshake{
		ProtocolVersion: protocolVersion,
		ServerAddress:   serverAddress,
		ServerPort:      serverPortBytes,
		NextState:       nextState,
	}, nil
}
