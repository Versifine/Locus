package protocol

import "io"

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
