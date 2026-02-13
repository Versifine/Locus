package protocol

import (
	"bytes"
	"testing"
)

func TestParseTileEntityDataWithoutNBT(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteInt64(buf, encodeBlockPosition(3, 70, -2))
	_ = WriteVarint(buf, 7)
	_ = WriteByte(buf, TagEnd) // optional NBT absent

	got, err := ParseTileEntityData(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ParseTileEntityData failed: %v", err)
	}
	if got.X != 3 || got.Y != 70 || got.Z != -2 {
		t.Fatalf("unexpected tile entity position: %+v", got)
	}
	if got.Action != 7 {
		t.Fatalf("unexpected action: got %d, want 7", got.Action)
	}
	if got.NBTData != nil {
		t.Fatalf("expected nil NBTData, got %+v", got.NBTData)
	}
}

func TestParseTileEntityDataWithNBT(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteInt64(buf, encodeBlockPosition(0, 64, 0))
	_ = WriteVarint(buf, 1)
	_ = WriteByte(buf, TagInt)
	_ = WriteInt32(buf, 42)

	got, err := ParseTileEntityData(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ParseTileEntityData failed: %v", err)
	}
	if got.NBTData == nil || got.NBTData.Type != TagInt {
		t.Fatalf("expected TagInt NBTData, got %+v", got.NBTData)
	}
	if v, ok := got.NBTData.Value.(int32); !ok || v != 42 {
		t.Fatalf("unexpected NBT value: %+v", got.NBTData.Value)
	}
}

func TestParseBlockAction(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteInt64(buf, encodeBlockPosition(-10, 64, 8))
	_ = WriteByte(buf, 1)
	_ = WriteByte(buf, 2)
	_ = WriteVarint(buf, 33)

	got, err := ParseBlockAction(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ParseBlockAction failed: %v", err)
	}
	if got.X != -10 || got.Y != 64 || got.Z != 8 {
		t.Fatalf("unexpected block action position: %+v", got)
	}
	if got.Byte1 != 1 || got.Byte2 != 2 || got.BlockID != 33 {
		t.Fatalf("unexpected block action payload: %+v", got)
	}
}

func TestParseBlockActionInvalidBlockID(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteInt64(buf, encodeBlockPosition(1, 64, 1))
	_ = WriteByte(buf, 0)
	_ = WriteByte(buf, 0)
	_ = WriteVarint(buf, -1)

	if _, err := ParseBlockAction(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatalf("expected error for negative block id")
	}
}
