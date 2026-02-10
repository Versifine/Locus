package protocol

import (
	"bytes"
	"testing"
)

func TestPlayerPosition(t *testing.T) {
	expected := &PlayerPosition{
		TeleportID: 42,
		X:          10.5,
		Y:          64.0,
		Z:          -20.2,
		Dx:         0.1,
		Dy:         0.2,
		Dz:         0.3,
		Yaw:        90.0,
		Pitch:      45.0,
		Flags:      0x01,
	}

	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, expected.TeleportID)
	_ = WriteDouble(buf, expected.X)
	_ = WriteDouble(buf, expected.Y)
	_ = WriteDouble(buf, expected.Z)
	_ = WriteDouble(buf, expected.Dx)
	_ = WriteDouble(buf, expected.Dy)
	_ = WriteDouble(buf, expected.Dz)
	_ = WriteFloat(buf, expected.Yaw)
	_ = WriteFloat(buf, expected.Pitch)
	_ = WriteInt32(buf, expected.Flags)

	parsed, err := ParsePlayerPosition(buf)
	if err != nil {
		t.Fatalf("ParsePlayerPosition failed: %v", err)
	}

	if parsed.TeleportID != expected.TeleportID {
		t.Errorf("TeleportID mismatch: expected %d, got %d", expected.TeleportID, parsed.TeleportID)
	}
	if parsed.X != expected.X || parsed.Y != expected.Y || parsed.Z != expected.Z {
		t.Errorf("Coordinates mismatch")
	}
	if parsed.Yaw != expected.Yaw || parsed.Pitch != expected.Pitch {
		t.Errorf("Rotation mismatch")
	}
	if parsed.Flags != expected.Flags {
		t.Errorf("Flags mismatch")
	}
}

func TestTeleportConfirm(t *testing.T) {
	teleportID := int32(123)
	packet := CreateTeleportConfirmPacket(teleportID)

	if packet.ID != C2STeleportConfirm {
		t.Errorf("Expected packet ID %d, got %d", C2STeleportConfirm, packet.ID)
	}

	buf := bytes.NewReader(packet.Payload)
	parsedID, err := ReadVarint(buf)
	if err != nil {
		t.Fatalf("ReadVarint failed: %v", err)
	}

	if parsedID != teleportID {
		t.Errorf("Expected TeleportID %d, got %d", teleportID, parsedID)
	}
}
