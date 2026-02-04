package protocol

import "errors"

var (
	ErrVarIntTooLong  = errors.New("varint is too long")
	ErrVarLongTooLong = errors.New("varlong is too long")
	ErrPacketTooLarge = errors.New("packet size exceeds maximum allowed")
	ErrInvalidPacket  = errors.New("invalid packet structure")
)
