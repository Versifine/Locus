package protocol

import "bytes"

type HandShake struct {
	ProtocolVersion int32
	ServerAddress   string
	ServerPort      uint16
	NextState       int32
}

func PerseHandShake(payload []byte) (*HandShake, error) {
	payloadReader := bytes.NewReader(payload)
	protocolVersion, err := ReadVarint(payloadReader)
	if err != nil {
		return nil, err
	}
	serverAddress, err := ReadString(payloadReader)
	if err != nil {
		return nil, err
	}
	serverPortBytes, err := ReadUnsignedShort(payloadReader)
	if err != nil {
		return nil, err
	}
	nextState, err := ReadVarint(payloadReader)
	if err != nil {
		return nil, err
	}

	return &HandShake{
		ProtocolVersion: protocolVersion,
		ServerAddress:   serverAddress,
		ServerPort:      serverPortBytes,
		NextState:       nextState,
	}, nil
}
