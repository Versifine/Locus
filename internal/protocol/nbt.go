package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
)

const (
	TagEnd       = 0
	TagByte      = 1
	TagShort     = 2
	TagInt       = 3
	TagLong      = 4
	TagFloat     = 5
	TagDouble    = 6
	TagByteArray = 7
	TagString    = 8
	TagList      = 9
	TagCompound  = 10
	TagIntArray  = 11
	TagLongArray = 12
)

type NBTNode struct {
	Type  byte
	Value any
}

func (n *NBTNode) String() string {
	switch n.Type {
	case TagByte:
		return fmt.Sprintf("Byte(%d)", n.Value.(byte))
	case TagShort:
		return fmt.Sprintf("Short(%d)", n.Value.(int16))
	case TagInt:
		return fmt.Sprintf("Int(%d)", n.Value.(int32))
	case TagLong:
		return fmt.Sprintf("Long(%d)", n.Value.(int64))
	case TagFloat:
		return fmt.Sprintf("Float(%f)", n.Value.(float32))
	case TagDouble:
		return fmt.Sprintf("Double(%f)", n.Value.(float64))
	case TagByteArray:
		return fmt.Sprintf("ByteArray(%v)", n.Value.([]byte))
	case TagString:
		return fmt.Sprintf("String(%s)", n.Value.(string))
	case TagList:
		return fmt.Sprintf("List(%v)", n.Value.([]*NBTNode))
	case TagCompound:
		return fmt.Sprintf("Compound(%v)", n.Value.(map[string]*NBTNode))
	case TagIntArray:
		return fmt.Sprintf("IntArray(%v)", n.Value.([]int32))
	case TagLongArray:
		return fmt.Sprintf("LongArray(%v)", n.Value.([]int64))
	default:
		return "Unknown"
	}
}

func ReadAnonymousNBT(r io.Reader) (*NBTNode, error) {
	typeByte, err := ReadByte(r)
	if err != nil {
		return nil, err
	}
	if typeByte == TagEnd {
		return &NBTNode{Type: TagEnd, Value: nil}, nil
	}
	return readPayload(r, typeByte)
}

func readPayload(r io.Reader, typeByte byte) (*NBTNode, error) {
	switch typeByte {
	case TagByte:
		b, err := NBTReadByte(r)
		if err != nil {
			return nil, err
		}
		return &NBTNode{Type: TagByte, Value: b}, nil
	case TagShort:
		s, err := NBTReadInt16(r)
		if err != nil {
			return nil, err
		}
		return &NBTNode{Type: TagShort, Value: s}, nil
	case TagInt:
		i, err := NBTReadInt32(r)
		if err != nil {
			return nil, err
		}
		return &NBTNode{Type: TagInt, Value: i}, nil
	case TagLong:
		l, err := NBTReadInt64(r)
		if err != nil {
			return nil, err
		}
		return &NBTNode{Type: TagLong, Value: l}, nil
	case TagFloat:
		f, err := NBTReadFloat32(r)
		if err != nil {
			return nil, err
		}
		return &NBTNode{Type: TagFloat, Value: f}, nil
	case TagDouble:
		d, err := NBTReadFloat64(r)
		if err != nil {
			return nil, err
		}
		return &NBTNode{Type: TagDouble, Value: d}, nil
	case TagByteArray:
		arr, err := NBTReadByteArray(r)
		if err != nil {
			return nil, err
		}
		return &NBTNode{Type: TagByteArray, Value: arr}, nil
	case TagString:
		s, err := NBTReadString(r)
		if err != nil {
			return nil, err
		}
		return &NBTNode{Type: TagString, Value: s}, nil
	case TagList:
		list, err := NBTReadList(r)
		if err != nil {
			return nil, err
		}
		return &NBTNode{Type: TagList, Value: list}, nil
	case TagCompound:
		compound, err := NBTReadCompound(r)
		if err != nil {
			return nil, err
		}
		return &NBTNode{Type: TagCompound, Value: compound}, nil
	case TagIntArray:
		arr, err := NBTReadIntArray(r)
		if err != nil {
			return nil, err
		}
		return &NBTNode{Type: TagIntArray, Value: arr}, nil
	case TagLongArray:
		arr, err := NBTReadLongArray(r)
		if err != nil {
			return nil, err
		}
		return &NBTNode{Type: TagLongArray, Value: arr}, nil
	default:
		// For simplicity, other tag types are not implemented here.
		slog.Warn("unsupported NBT tag type", "type", typeByte)
		return nil, fmt.Errorf("unsupported NBT tag type: %d", typeByte)

	}
}
func NBTReadByte(r io.Reader) (byte, error) {
	var buf [1]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}
func NBTReadInt16(r io.Reader) (int16, error) {
	var buf [2]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int16(binary.BigEndian.Uint16(buf[:])), nil
}
func NBTReadInt32(r io.Reader) (int32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(buf[:])), nil
}
func NBTReadInt64(r io.Reader) (int64, error) {
	var buf [8]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(buf[:])), nil
}
func NBTReadFloat32(r io.Reader) (float32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	bits := binary.BigEndian.Uint32(buf[:])
	return math.Float32frombits(bits), nil
}

func NBTReadFloat64(r io.Reader) (float64, error) {
	var buf [8]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	bits := binary.BigEndian.Uint64(buf[:])
	return math.Float64frombits(bits), nil
}

func NBTReadByteArray(r io.Reader) ([]byte, error) {
	length, err := NBTReadInt32(r)
	if err != nil {
		return nil, err
	}
	data := make([]byte, length)
	_, err = io.ReadFull(r, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func NBTReadString(r io.Reader) (string, error) {
	length, err := ReadUnsignedShort(r)
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
func NBTReadIntArray(r io.Reader) ([]int32, error) {
	length, err := NBTReadInt32(r)
	if err != nil {
		return nil, err
	}
	data := make([]int32, length)
	for i := int32(0); i < length; i++ {
		val, err := NBTReadInt32(r)
		if err != nil {
			return nil, err
		}
		data[i] = val
	}
	return data, nil
}
func NBTReadLongArray(r io.Reader) ([]int64, error) {
	length, err := NBTReadInt32(r)
	if err != nil {
		return nil, err
	}
	data := make([]int64, length)
	for i := int32(0); i < length; i++ {
		val, err := NBTReadInt64(r)
		if err != nil {
			return nil, err
		}
		data[i] = val
	}
	return data, nil
}
func NBTReadList(r io.Reader) ([]*NBTNode, error) {
	elementType, err := NBTReadByte(r)
	if err != nil {
		return nil, err
	}
	length, err := NBTReadInt32(r)
	if err != nil {
		return nil, err
	}
	list := make([]*NBTNode, length)
	for i := int32(0); i < length; i++ {
		element, err := readPayload(r, elementType)
		if err != nil {
			return nil, err
		}
		list[i] = element
	}
	return list, nil
}
func NBTReadCompound(r io.Reader) (map[string]*NBTNode, error) {
	compound := make(map[string]*NBTNode)
	for {
		typeByte, err := NBTReadByte(r)
		if err != nil {
			return nil, err
		}
		if typeByte == TagEnd {
			break
		}
		name, err := NBTReadString(r)
		if err != nil {
			return nil, err
		}
		payload, err := readPayload(r, typeByte)
		if err != nil {
			return nil, err
		}
		compound[name] = payload
	}
	return compound, nil
}
