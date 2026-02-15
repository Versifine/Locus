package protocol

import (
	"bytes"
	"testing"
)

func TestCreatePlayerInputPacket(t *testing.T) {
	packet := CreatePlayerInputPacket(true, false, true, false, true, true, false)
	if packet.ID != C2SPlayerInput {
		t.Fatalf("packet.ID = %d, want %d", packet.ID, C2SPlayerInput)
	}

	r := bytes.NewReader(packet.Payload)
	flags, err := ReadByte(r)
	if err != nil {
		t.Fatalf("ReadByte(flags) failed: %v", err)
	}

	// forward(1) + left(4) + jump(16) + shift(32) = 53
	if flags != 0x35 {
		t.Fatalf("flags = 0x%02x, want 0x35", flags)
	}
}
