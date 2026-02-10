package protocol

import "io"

type UpdateHealth struct {
	Health         float32
	Food           int32
	FoodSaturation float32
}
type UpdateTime struct {
	Age         int64
	WorldTime   int64
	TickDayTime bool
}

func ParseUpdateHealth(r io.Reader) (*UpdateHealth, error) {
	health, err := ReadFloat(r)
	if err != nil {
		return nil, err
	}
	food, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	foodSaturation, err := ReadFloat(r)
	if err != nil {
		return nil, err
	}

	return &UpdateHealth{
		Health:         health,
		Food:           int32(food),
		FoodSaturation: foodSaturation,
	}, nil
}

func ParseUpdateTime(r io.Reader) (*UpdateTime, error) {
	age, err := ReadInt64(r)
	if err != nil {
		return nil, err
	}
	worldTime, err := ReadInt64(r)
	if err != nil {
		return nil, err
	}
	tickDayTime, err := ReadBool(r)
	if err != nil {
		return nil, err
	}
	return &UpdateTime{
		Age:         age,
		WorldTime:   worldTime,
		TickDayTime: tickDayTime,
	}, nil
}
