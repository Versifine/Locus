package protocol

import "bytes"

const (
	playerInputForwardBit  = 1 << 0
	playerInputBackwardBit = 1 << 1
	playerInputLeftBit     = 1 << 2
	playerInputRightBit    = 1 << 3
	playerInputJumpBit     = 1 << 4
	playerInputShiftBit    = 1 << 5
	playerInputSprintBit   = 1 << 6
)

func CreatePlayerInputPacket(forward, backward, left, right, jump, shift, sprint bool) *Packet {
	var flags byte
	if forward {
		flags |= playerInputForwardBit
	}
	if backward {
		flags |= playerInputBackwardBit
	}
	if left {
		flags |= playerInputLeftBit
	}
	if right {
		flags |= playerInputRightBit
	}
	if jump {
		flags |= playerInputJumpBit
	}
	if shift {
		flags |= playerInputShiftBit
	}
	if sprint {
		flags |= playerInputSprintBit
	}

	buf := new(bytes.Buffer)
	_ = WriteByte(buf, flags)
	return &Packet{
		ID:      C2SPlayerInput,
		Payload: buf.Bytes(),
	}
}
