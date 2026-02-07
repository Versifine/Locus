package protocol

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"math"
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
			err = ErrVarIntTooLong
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
			err = ErrVarLongTooLong
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
func WriteString(w io.Writer, s string) error {
	err := WriteVarint(w, int32(len(s)))
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(s))
	return err
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

func (u UUID) String() string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

func ReadUUID(r io.Reader) (UUID, error) {
	var buf [16]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return UUID{}, err
	}
	return UUID(buf), nil
}

func ReadBool(r io.Reader) (bool, error) {
	var buf [1]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return false, err
	}
	return buf[0] != 0, nil
}

func WriteUUID(w io.Writer, uuid UUID) error {
	_, err := w.Write(uuid[:])
	return err
}

func WriteUnsignedShort(w io.Writer, value uint16) error {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], value)
	_, err := w.Write(buf[:])
	return err
}

func WriteBool(w io.Writer, value bool) error {
	var b byte
	if value {
		b = 1
	}
	_, err := w.Write([]byte{b})
	return err
}

func WriteInt64(w io.Writer, value int64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(value))
	_, err := w.Write(buf[:])
	return err
}

func WriteFloat(w io.Writer, value float32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], math.Float32bits(value))
	_, err := w.Write(buf[:])
	return err
}

func WriteDouble(w io.Writer, value float64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], math.Float64bits(value))
	_, err := w.Write(buf[:])
	return err
}

// GenerateOfflineUUID generates a version-3 UUID for offline-mode players.
// Algorithm: MD5("OfflinePlayer:" + username), then set version=3 and variant=RFC4122.
func GenerateOfflineUUID(username string) UUID {
	hash := md5.Sum([]byte("OfflinePlayer:" + username))
	// Set version to 3: byte 6 → 0011xxxx
	hash[6] = (hash[6] & 0x0F) | 0x30
	// Set variant to RFC 4122: byte 8 → 10xxxxxx
	hash[8] = (hash[8] & 0x3F) | 0x80
	return UUID(hash)
}
