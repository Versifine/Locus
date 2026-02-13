package protocol

import (
	"bytes"
	"testing"
)

func TestCreatePlayerPositionPacket(t *testing.T) {
	packet := CreatePlayerPositionPacket(10.5, 64.0, -20.25, true, false)
	if packet.ID != C2SPlayerPosition {
		t.Fatalf("packet.ID = %d, want %d", packet.ID, C2SPlayerPosition)
	}

	r := bytes.NewReader(packet.Payload)
	x, err := ReadDouble(r)
	if err != nil {
		t.Fatalf("ReadDouble(x) failed: %v", err)
	}
	y, err := ReadDouble(r)
	if err != nil {
		t.Fatalf("ReadDouble(y) failed: %v", err)
	}
	z, err := ReadDouble(r)
	if err != nil {
		t.Fatalf("ReadDouble(z) failed: %v", err)
	}
	flags, err := ReadByte(r)
	if err != nil {
		t.Fatalf("ReadByte(flags) failed: %v", err)
	}

	if x != 10.5 || y != 64.0 || z != -20.25 {
		t.Fatalf("unexpected coords: (%f,%f,%f)", x, y, z)
	}
	if flags != 0x01 {
		t.Fatalf("flags = 0x%02x, want 0x01", flags)
	}
}

func TestCreatePlayerPositionAndRotationPacket(t *testing.T) {
	packet := CreatePlayerPositionAndRotationPacket(1.0, 2.0, 3.0, 90.0, -30.0, false, true)
	if packet.ID != C2SPlayerPositionLook {
		t.Fatalf("packet.ID = %d, want %d", packet.ID, C2SPlayerPositionLook)
	}

	r := bytes.NewReader(packet.Payload)
	x, err := ReadDouble(r)
	if err != nil {
		t.Fatalf("ReadDouble(x) failed: %v", err)
	}
	y, err := ReadDouble(r)
	if err != nil {
		t.Fatalf("ReadDouble(y) failed: %v", err)
	}
	z, err := ReadDouble(r)
	if err != nil {
		t.Fatalf("ReadDouble(z) failed: %v", err)
	}
	yaw, err := ReadFloat(r)
	if err != nil {
		t.Fatalf("ReadFloat(yaw) failed: %v", err)
	}
	pitch, err := ReadFloat(r)
	if err != nil {
		t.Fatalf("ReadFloat(pitch) failed: %v", err)
	}
	flags, err := ReadByte(r)
	if err != nil {
		t.Fatalf("ReadByte(flags) failed: %v", err)
	}

	if x != 1.0 || y != 2.0 || z != 3.0 || yaw != 90.0 || pitch != -30.0 {
		t.Fatalf("unexpected payload values: x=%f y=%f z=%f yaw=%f pitch=%f", x, y, z, yaw, pitch)
	}
	if flags != 0x02 {
		t.Fatalf("flags = 0x%02x, want 0x02", flags)
	}
}

func TestCreatePlayerLoadedPacket(t *testing.T) {
	packet := CreatePlayerLoadedPacket()
	if packet.ID != C2SPlayerLoaded {
		t.Fatalf("packet.ID = %d, want %d", packet.ID, C2SPlayerLoaded)
	}
	if len(packet.Payload) != 0 {
		t.Fatalf("payload len = %d, want 0", len(packet.Payload))
	}
}
