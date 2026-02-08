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
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, protocolVersion)
	_ = WriteString(buf, serverAddress)
	_ = WriteUnsignedShort(buf, serverPort)
	_ = WriteVarint(buf, nextState)
	return &Packet{
		ID:      C2SHandshake,
		Payload: buf.Bytes(),
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
