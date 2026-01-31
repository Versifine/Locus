// Package protocol 负责 Minecraft 协议的编码
package protocol

import (
	"encoding/binary"
	"io"
)

// WriteVarInt 向 writer 写入一个 VarInt
func WriteVarInt(w io.Writer, value int32) error {
	for {
		temp := byte(value & 0x7F)
		value >>= 7
		if value != 0 {
			temp |= 0x80
		}
		_, err := w.Write([]byte{temp})
		if err != nil {
			return err
		}
		if value == 0 {
			break
		}
	}
	return nil
}

// WriteString 向 writer 写入一个带长度前缀的字符串
func WriteString(w io.Writer, s string) error {
	data := []byte(s)
	if err := WriteVarInt(w, int32(len(data))); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// WriteUint16 向 writer 写入一个 uint16（大端序）
func WriteUint16(w io.Writer, value uint16) error {
	return binary.Write(w, binary.BigEndian, value)
}

// VarIntLen 返回 VarInt 编码后的字节长度
func VarIntLen(value int32) int {
	count := 0
	for {
		count++
		value >>= 7
		if value == 0 {
			break
		}
	}
	return count
}
