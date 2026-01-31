// Package protocol 负责 Minecraft 协议的解码
package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

var ErrVarIntTooLarge = errors.New("VarInt 过大")

// ReadVarInt 从 reader 中读取一个 VarInt
func ReadVarInt(r io.Reader) (int32, error) {
	var result int32
	var numRead uint

	for {
		buf := make([]byte, 1)
		_, err := r.Read(buf)
		if err != nil {
			return 0, err
		}

		value := int32(buf[0] & 0x7F)
		result |= value << (7 * numRead)

		numRead++
		if numRead > 5 {
			return 0, ErrVarIntTooLarge
		}

		if buf[0]&0x80 == 0 {
			break
		}
	}

	return result, nil
}

// ReadString 从 reader 中读取一个带长度前缀的字符串
func ReadString(r io.Reader) (string, error) {
	length, err := ReadVarInt(r)
	if err != nil {
		return "", err
	}

	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

// ReadUint16 从 reader 中读取一个 uint16（大端序）
func ReadUint16(r io.Reader) (uint16, error) {
	var value uint16
	err := binary.Read(r, binary.BigEndian, &value)
	return value, err
}
