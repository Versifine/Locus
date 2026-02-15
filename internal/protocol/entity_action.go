package protocol

import "bytes"

const (
	EntityActionStartSneaking  = 0
	EntityActionStopSneaking   = 1
	EntityActionStartSprinting = 3
	EntityActionStopSprinting  = 4
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
