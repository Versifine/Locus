package protocol

import (
	"bytes"
	"testing"
)

func TestParseEntityMetadataItemSlot_Found(t *testing.T) {
	var payload bytes.Buffer

	_ = WriteVarint(&payload, 123)
	writeMetadataEntry(&payload, 0, 0, func(buf *bytes.Buffer) { _ = WriteByte(buf, 0x01) })
	writeMetadataEntry(&payload, 2, 6, func(buf *bytes.Buffer) { _ = WriteBool(buf, false) })
	writeMetadataEntry(&payload, 8, 7, func(buf *bytes.Buffer) {
		_ = WriteVarint(buf, 1)    // itemCount
		_ = WriteVarint(buf, 1031) // egg
		_ = WriteVarint(buf, 0)    // addedComponentCount
		_ = WriteVarint(buf, 0)    // removedComponentCount
	})
	_ = WriteByte(&payload, 0xFF)

	entityID, itemID, found, err := ParseEntityMetadataItemSlot(bytes.NewReader(payload.Bytes()))
	if err != nil {
		t.Fatalf("ParseEntityMetadataItemSlot() error = %v", err)
	}
	if !found {
		t.Fatalf("ParseEntityMetadataItemSlot() found = false, want true")
	}
	if entityID != 123 {
		t.Fatalf("ParseEntityMetadataItemSlot() entityID = %d, want 123", entityID)
	}
	if itemID != 1031 {
		t.Fatalf("ParseEntityMetadataItemSlot() itemID = %d, want 1031", itemID)
	}
}

func TestParseEntityMetadataItemSlot_NotFound(t *testing.T) {
	var payload bytes.Buffer

	_ = WriteVarint(&payload, 42)
	writeMetadataEntry(&payload, 0, 0, func(buf *bytes.Buffer) { _ = WriteByte(buf, 0x00) })
	writeMetadataEntry(&payload, 1, 1, func(buf *bytes.Buffer) { _ = WriteVarint(buf, 100) })
	_ = WriteByte(&payload, 0xFF)

	entityID, itemID, found, err := ParseEntityMetadataItemSlot(bytes.NewReader(payload.Bytes()))
	if err != nil {
		t.Fatalf("ParseEntityMetadataItemSlot() error = %v", err)
	}
	if found {
		t.Fatalf("ParseEntityMetadataItemSlot() found = true, want false")
	}
	if entityID != 42 {
		t.Fatalf("ParseEntityMetadataItemSlot() entityID = %d, want 42", entityID)
	}
	if itemID != 0 {
		t.Fatalf("ParseEntityMetadataItemSlot() itemID = %d, want 0", itemID)
	}
}

func TestParseEntityMetadataItemSlot_EmptySlot(t *testing.T) {
	var payload bytes.Buffer

	_ = WriteVarint(&payload, 9)
	writeMetadataEntry(&payload, 8, 7, func(buf *bytes.Buffer) {
		_ = WriteVarint(buf, 0) // empty slot
	})
	_ = WriteByte(&payload, 0xFF)

	entityID, itemID, found, err := ParseEntityMetadataItemSlot(bytes.NewReader(payload.Bytes()))
	if err != nil {
		t.Fatalf("ParseEntityMetadataItemSlot() error = %v", err)
	}
	if found {
		t.Fatalf("ParseEntityMetadataItemSlot() found = true, want false")
	}
	if entityID != 9 {
		t.Fatalf("ParseEntityMetadataItemSlot() entityID = %d, want 9", entityID)
	}
	if itemID != 0 {
		t.Fatalf("ParseEntityMetadataItemSlot() itemID = %d, want 0", itemID)
	}
}

func TestParseEntityMetadataItemSlot_WithOptionalComponentNBT(t *testing.T) {
	var payload bytes.Buffer

	_ = WriteVarint(&payload, 77)
	writeMetadataEntry(&payload, 2, 6, func(buf *bytes.Buffer) {
		_ = WriteBool(buf, true)
		_ = WriteByte(buf, TagEnd) // empty anonymous NBT
	})
	writeMetadataEntry(&payload, 8, 7, func(buf *bytes.Buffer) {
		_ = WriteVarint(buf, 1)
		_ = WriteVarint(buf, 1031)
		_ = WriteVarint(buf, 0)
		_ = WriteVarint(buf, 0)
	})
	_ = WriteByte(&payload, 0xFF)

	entityID, itemID, found, err := ParseEntityMetadataItemSlot(bytes.NewReader(payload.Bytes()))
	if err != nil {
		t.Fatalf("ParseEntityMetadataItemSlot() error = %v", err)
	}
	if !found {
		t.Fatalf("ParseEntityMetadataItemSlot() found = false, want true")
	}
	if entityID != 77 {
		t.Fatalf("ParseEntityMetadataItemSlot() entityID = %d, want 77", entityID)
	}
	if itemID != 1031 {
		t.Fatalf("ParseEntityMetadataItemSlot() itemID = %d, want 1031", itemID)
	}
}

func TestParseEntityMetadataItemSlot_WithPoseBeforeSlot(t *testing.T) {
	var payload bytes.Buffer

	_ = WriteVarint(&payload, 88)
	writeMetadataEntry(&payload, 6, 20, func(buf *bytes.Buffer) {
		_ = WriteVarint(buf, 0) // standing pose
	})
	writeMetadataEntry(&payload, 7, 1, func(buf *bytes.Buffer) {
		_ = WriteVarint(buf, 0) // ticks frozen
	})
	writeMetadataEntry(&payload, 8, 7, func(buf *bytes.Buffer) {
		_ = WriteVarint(buf, 1)
		_ = WriteVarint(buf, 264) // Diamond
		_ = WriteVarint(buf, 0)
		_ = WriteVarint(buf, 0)
	})
	_ = WriteByte(&payload, 0xFF)

	entityID, itemID, found, err := ParseEntityMetadataItemSlot(bytes.NewReader(payload.Bytes()))
	if err != nil {
		t.Fatalf("ParseEntityMetadataItemSlot() error = %v", err)
	}
	if !found {
		t.Fatalf("ParseEntityMetadataItemSlot() found = false, want true")
	}
	if entityID != 88 {
		t.Fatalf("ParseEntityMetadataItemSlot() entityID = %d, want 88", entityID)
	}
	if itemID != 264 {
		t.Fatalf("ParseEntityMetadataItemSlot() itemID = %d, want 264", itemID)
	}
}

func TestParseEntityMetadataItemSlot_UnknownTypeGraceful(t *testing.T) {
	var payload bytes.Buffer

	_ = WriteVarint(&payload, 5)
	writeMetadataEntry(&payload, 1, 16, func(buf *bytes.Buffer) {})

	entityID, itemID, found, err := ParseEntityMetadataItemSlot(bytes.NewReader(payload.Bytes()))
	if err != nil {
		t.Fatalf("ParseEntityMetadataItemSlot() error = %v", err)
	}
	if found {
		t.Fatalf("ParseEntityMetadataItemSlot() found = true, want false")
	}
	if entityID != 5 {
		t.Fatalf("ParseEntityMetadataItemSlot() entityID = %d, want 5", entityID)
	}
	if itemID != 0 {
		t.Fatalf("ParseEntityMetadataItemSlot() itemID = %d, want 0", itemID)
	}
}

func writeMetadataEntry(buf *bytes.Buffer, key byte, metaType int32, writeValue func(*bytes.Buffer)) {
	_ = WriteByte(buf, key)
	_ = WriteVarint(buf, metaType)
	writeValue(buf)
}
