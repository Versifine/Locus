package protocol

import (
	"bytes"
	"testing"
)

func TestParsePlayerInfo_AddPlayer(t *testing.T) {
	// Action 0x01: Add Player
	// We only check if it correctly parses the UUID and Name when action 0x01 is set.

	uuid := UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	name := "TestPlayer"

	buf := new(bytes.Buffer)
	_ = WriteByte(buf, 0x01) // Action: Add Player
	_ = WriteVarint(buf, 1)  // Player Count
	_ = WriteUUID(buf, uuid)
	_ = WriteString(buf, name)
	_ = WriteVarint(buf, 0) // Property count: 0

	parsed, err := ParsePlayerInfo(buf)
	if err != nil {
		t.Fatalf("ParsePlayerInfo failed: %v", err)
	}

	if parsed.PlayerCount != 1 {
		t.Errorf("Expected player count 1, got %d", parsed.PlayerCount)
	}
	if parsed.Players[0].UUID != uuid {
		t.Errorf("UUID mismatch")
	}
	if parsed.Players[0].Name != name {
		t.Errorf("Name mismatch: expected %s, got %s", name, parsed.Players[0].Name)
	}
}

func TestParsePlayerInfo_OtherActions(t *testing.T) {
	// Testing simple bit flag handling for other actions to ensure it doesn't crash
	buf := new(bytes.Buffer)
	_ = WriteByte(buf, 0x04) // Action: Update Listed (0x04)
	_ = WriteVarint(buf, 1)  // Player Count
	uuid := UUID{0x01}
	_ = WriteUUID(buf, uuid)
	_ = WriteVarint(buf, 1) // Listed: true (Varint)

	parsed, err := ParsePlayerInfo(buf)
	if err != nil {
		t.Fatalf("ParsePlayerInfo failed: %v", err)
	}

	if parsed.Actions != 0x04 {
		t.Errorf("Expected action 0x04, got 0x%x", parsed.Actions)
	}
}
