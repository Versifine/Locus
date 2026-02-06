package protocol

import "errors"

var (
	ErrVarIntTooLong    = errors.New("varint is too long")
	ErrVarLongTooLong   = errors.New("varlong is too long")
	ErrPacketTooLarge   = errors.New("packet size exceeds maximum allowed")
	ErrInvalidPacket    = errors.New("invalid packet structure")
	ErrInvalidNBTType   = errors.New("invalid NBT type")
	ErrMissingField     = errors.New("missing required field in NBT compound")
	ErrInvalidFieldType = errors.New("invalid field type in NBT compound")
)
