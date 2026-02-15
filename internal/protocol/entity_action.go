package protocol

import "bytes"

const (
	EntityActionLeaveBed       = 0
	EntityActionStartSprinting = 1
	EntityActionStopSprinting  = 2
	EntityActionStartHorseJump = 3
	EntityActionStopHorseJump  = 4
	EntityActionOpenVehicleInv = 5
	EntityActionStartElytraFly = 6
)

func CreateEntityActionPacket(entityID, actionID, jumpBoost int32) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, entityID)
	_ = WriteVarint(buf, actionID)
	_ = WriteVarint(buf, jumpBoost)
	return &Packet{
		ID:      C2SEntityAction,
		Payload: buf.Bytes(),
	}
}
