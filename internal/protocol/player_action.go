package protocol

import (
	"bytes"
	"encoding/binary"
)

const (
	UseEntityActionInteract   int32 = 0
	UseEntityActionAttack     int32 = 1
	UseEntityActionInteractAt int32 = 2
)

func CreateHeldItemSlotPacket(slot int16) *Packet {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.BigEndian, slot)
	return &Packet{ID: C2SHeldItemSlot, Payload: buf.Bytes()}
}

func CreateUseEntityPacket(
	entityID int32,
	actionType int32,
	targetX *float32,
	targetY *float32,
	targetZ *float32,
	hand *int32,
	sneaking bool,
) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, entityID)
	_ = WriteVarint(buf, actionType)

	if actionType == UseEntityActionInteractAt {
		_ = WriteFloat(buf, derefFloat32(targetX))
		_ = WriteFloat(buf, derefFloat32(targetY))
		_ = WriteFloat(buf, derefFloat32(targetZ))
	}
	if actionType == UseEntityActionInteract || actionType == UseEntityActionInteractAt {
		_ = WriteVarint(buf, derefInt32(hand))
	}

	_ = WriteBool(buf, sneaking)

	return &Packet{ID: C2SUseEntity, Payload: buf.Bytes()}
}

func CreateUseItemPacket(hand int32, sequence int32) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, hand)
	_ = WriteVarint(buf, sequence)
	_ = WriteFloat(buf, 0)
	_ = WriteFloat(buf, 0)
	return &Packet{ID: C2SUseItem, Payload: buf.Bytes()}
}

func CreateArmAnimationPacket(hand int32) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, hand)
	return &Packet{ID: C2SArmAnimation, Payload: buf.Bytes()}
}

func CreateBlockPlacePacket(
	pos BlockPos,
	face int,
	hand int,
	cursorX float32,
	cursorY float32,
	cursorZ float32,
	insideBlock bool,
	worldBorderHit bool,
	sequence int32,
) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, int32(hand))
	_ = WriteInt64(buf, encodeBlockPosition(pos.X, pos.Y, pos.Z))
	_ = WriteVarint(buf, int32(face))
	_ = WriteFloat(buf, cursorX)
	_ = WriteFloat(buf, cursorY)
	_ = WriteFloat(buf, cursorZ)
	_ = WriteBool(buf, insideBlock)
	_ = WriteBool(buf, worldBorderHit)
	_ = WriteVarint(buf, sequence)
	return &Packet{ID: C2SBlockPlace, Payload: buf.Bytes()}
}

func derefFloat32(v *float32) float32 {
	if v == nil {
		return 0
	}
	return *v
}

func derefInt32(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}
