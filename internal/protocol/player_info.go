package protocol

import "io"

type PlayerInfo struct {
	Actions     uint8
	PlayerCount int32
	Players     []Player
}
type Player struct {
	UUID UUID
	Name string
}

func ParsePlayerInfo(r io.Reader) (*PlayerInfo, error) {

	actions, err := ReadByte(r)
	if err != nil {
		return nil, err
	}
	playerCount, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	players := make([]Player, playerCount)
	for i := int32(0); i < playerCount; i++ {
		uuid, err := ReadUUID(r)
		if err != nil {
			return nil, err
		}
		players[i].UUID = uuid
		if actions&0x01 != 0 {
			name, err := ReadString(r)
			if err != nil {
				return nil, err
			}
			count, err := ReadVarint(r)
			if err != nil {
				return nil, err
			}
			for j := int32(0); j < count; j++ {
				_, err := ReadString(r)
				if err != nil {
					return nil, err
				}
				_, err = ReadString(r)
				if err != nil {
					return nil, err
				}
				isSigned, err := ReadBool(r)
				if err != nil {
					return nil, err
				}
				if isSigned {
					_, err = ReadString(r)
					if err != nil {
						return nil, err
					}
				}
			}
			players[i].Name = name
		}
		if actions&0x02 != 0 {
			hasSession, err := ReadBool(r)
			if err != nil {
				return nil, err
			}
			if hasSession {
				_, err := ReadUUID(r)
				if err != nil {
					return nil, err
				}
				_, err = ReadInt64(r)
				if err != nil {
					return nil, err
				}
				n, err := ReadVarint(r)
				if err != nil {
					return nil, err
				}
				for j := int32(0); j < n; j++ {
					_, err := ReadByte(r)
					if err != nil {
						return nil, err
					}
				}
				m, err := ReadVarint(r)
				if err != nil {
					return nil, err
				}
				for j := int32(0); j < m; j++ {
					_, err := ReadByte(r)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		if actions&0x04 != 0 {
			_, err := ReadVarint(r)
			if err != nil {
				return nil, err
			}
		}
		if actions&0x08 != 0 {
			_, err := ReadVarint(r)
			if err != nil {
				return nil, err
			}
		}
		if actions&0x10 != 0 {
			_, err := ReadVarint(r)
			if err != nil {
				return nil, err
			}
		}
		if actions&0x20 != 0 {
			flag, err := ReadBool(r)
			if err != nil {
				return nil, err
			}
			if flag {
				_, err := ReadAnonymousNBT(r)
				if err != nil {
					return nil, err
				}
			}
		}
		if actions&0x80 != 0 {
			_, err := ReadVarint(r)
			if err != nil {
				return nil, err
			}
		}
		if actions&0x40 != 0 {
			_, err := ReadBool(r)
			if err != nil {
				return nil, err
			}
		}
	}
	return &PlayerInfo{
		Actions:     actions,
		PlayerCount: playerCount,
		Players:     players,
	}, nil
}
