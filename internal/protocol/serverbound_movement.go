package protocol

import "bytes"

const (
	movementFlagOnGround               = 0x01
	movementFlagHasHorizontalCollision = 0x02
)

func CreatePlayerPositionPacket(x, y, z float64, onGround bool) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteDouble(buf, x)
	_ = WriteDouble(buf, y)
	_ = WriteDouble(buf, z)
	_ = WriteByte(buf, encodeMovementFlags(onGround, false))
	return &Packet{
		ID:      C2SPlayerPosition,
		Payload: buf.Bytes(),
	}
}

func CreatePlayerRotationPacket(yaw, pitch float32, onGround bool) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteFloat(buf, yaw)
	_ = WriteFloat(buf, pitch)
	_ = WriteByte(buf, encodeMovementFlags(onGround, false))
	return &Packet{
		ID:      C2SPlayerRotation,
		Payload: buf.Bytes(),
	}
}

func CreatePlayerPositionAndRotationPacket(x, y, z float64, yaw, pitch float32, onGround bool) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteDouble(buf, x)
	_ = WriteDouble(buf, y)
	_ = WriteDouble(buf, z)
	_ = WriteFloat(buf, yaw)
	_ = WriteFloat(buf, pitch)
	_ = WriteByte(buf, encodeMovementFlags(onGround, false))
	return &Packet{
		ID:      C2SPlayerPositionLook,
		Payload: buf.Bytes(),
	}
}

func encodeMovementFlags(onGround, hasHorizontalCollision bool) byte {
	var flags byte
	if onGround {
		flags |= movementFlagOnGround
	}
	if hasHorizontalCollision {
		flags |= movementFlagHasHorizontalCollision
	}
	return flags
}
