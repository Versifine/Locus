package protocol

import (
	"bytes"
	"testing"
)

func TestKeepAlive(t *testing.T) {
	keepAliveID := int64(12345678)
	packetID := int32(0x12)

	// Test CreateKeepAlivePacket
	packet := CreateKeepAlivePacket(keepAliveID, packetID)
	if packet.ID != packetID {
		t.Errorf("Expected packet ID %d, got %d", packetID, packet.ID)
	}

	// Test ParseKeepAlive
	buf := bytes.NewReader(packet.Payload)
	parsed, err := ParseKeepAlive(buf)
	if err != nil {
		t.Fatalf("ParseKeepAlive failed: %v", err)
	}

	if parsed.KeepAliveID != keepAliveID {
		t.Errorf("Expected KeepAliveID %d, got %d", keepAliveID, parsed.KeepAliveID)
	}
}
