package protocol

import "io"

type PlayerRemove struct {
	PlayerCount int32
	Players     []UUID
}

func ParsePlayerRemove(r io.Reader) (*PlayerRemove, error) {
	playerCount, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	players := make([]UUID, playerCount)
	for i := int32(0); i < playerCount; i++ {
		uuid, err := ReadUUID(r)
		if err != nil {
			return nil, err
		}
		players[i] = uuid
	}
	return &PlayerRemove{
		PlayerCount: playerCount,
		Players:     players,
	}, nil
}
