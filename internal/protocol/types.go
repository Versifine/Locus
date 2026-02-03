package protocol

import (
	"encoding/binary"
	"io"
)

const (
	SEGMENT_BITS = 0x7F
	CONTINUE_BIT = 0x80
)

func ReadVarint(r io.Reader) (value int32, err error) {
	value = 0
	position := 0
	currentByte := make([]byte, 1)
	for {
		_, err = r.Read(currentByte)
		if err != nil {
			return
		}
		b := currentByte[0]
		value |= int32(b&SEGMENT_BITS) << position
		if (b & CONTINUE_BIT) == 0 {
			break
		}
		position += 7
		if position >= 32 {
			err = io.ErrUnexpectedEOF
			return
		}
	}
	return
}

func WriteVarint(w io.Writer, value int32) (err error) {
	uvalue := uint32(value)
	for {
		temp := byte(uvalue & SEGMENT_BITS)
		uvalue >>= 7
		if uvalue != 0 {
			temp |= CONTINUE_BIT
		}
		_, err = w.Write([]byte{temp})
		if err != nil {
			return
		}
		if uvalue == 0 {
			break
		}
	}
	return
}

func ReadVarLong(r io.Reader) (value int64, err error) {
	value = 0
	position := 0
	currentByte := make([]byte, 1)
	for {
		_, err = r.Read(currentByte)
		if err != nil {
			return
		}
		b := currentByte[0]
		value |= int64(b&SEGMENT_BITS) << position
		if (b & CONTINUE_BIT) == 0 {
			break
		}
		position += 7
		if position >= 64 {
			err = io.ErrUnexpectedEOF
			return
		}
	}
	return
}
func WriteVarLong(w io.Writer, value int64) (err error) {
	uvalue := uint64(value)
	for {
		temp := byte(uvalue & SEGMENT_BITS)
		uvalue >>= 7
		if uvalue != 0 {
			temp |= CONTINUE_BIT
		}
		_, err = w.Write([]byte{temp})
		if err != nil {
			return
		}
		if uvalue == 0 {
			break
		}
	}
	return
}

func ReadString(r io.Reader) (string, error) {
	length, err := ReadVarint(r)
	if err != nil {
		return "", err
	}
	strBytes := make([]byte, length)
	_, err = io.ReadFull(r, strBytes)
	if err != nil {
		return "", err
	}
	return string(strBytes), nil
}

func ReadUnsignedShort(r io.Reader) (uint16, error) {
	var buf [2]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf[:]), nil
}

type UUID [16]byte

func ReadUUID(r io.Reader) (UUID, error) {
	var buf [16]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return UUID{}, err
	}
	return UUID(buf), nil
}
