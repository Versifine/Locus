package protocol

import (
	"bytes"
	"io"
)

const (
	RelX        = 0x001
	RelY        = 0x002
	RelZ        = 0x004
	RelYaw      = 0x008
	RelPitch    = 0x010
	RelVelX     = 0x020
	RelVelY     = 0x040
	RelVelZ     = 0x080
	RelRotDelta = 0x100
)

type PlayerPosition struct {
	TeleportID int32
	X          float64
	Y          float64
	Z          float64
	Dx         float64
	Dy         float64
	Dz         float64
	Yaw        float32
	Pitch      float32
	Flags      int32
}

func ParsePlayerPosition(r io.Reader) (*PlayerPosition, error) {
	teleportID, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	x, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	y, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	z, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	dx, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	dy, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	dz, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	yaw, err := ReadFloat(r)
	if err != nil {
		return nil, err
	}
	pitch, err := ReadFloat(r)
	if err != nil {
		return nil, err
	}
	flags, err := ReadInt32(r)
	if err != nil {
		return nil, err
	}
	return &PlayerPosition{
		TeleportID: teleportID,
		X:          x,
		Y:          y,
		Z:          z,
		Dx:         dx,
		Dy:         dy,
		Dz:         dz,
		Yaw:        yaw,
		Pitch:      pitch,
		Flags:      flags,
	}, nil
}

type TeleportConfirm struct {
	TeleportID int32
}

func CreateTeleportConfirmPacket(teleportID int32) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, teleportID)
	return &Packet{
		ID:      C2STeleportConfirm,
		Payload: buf.Bytes(),
	}
}
