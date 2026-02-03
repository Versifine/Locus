package protocol

import "io"

type Handshake struct {
	ProtocolVersion int32
	ServerAddress   string
	ServerPort      uint16
	NextState       int32
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
