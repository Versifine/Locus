package protocol

import (
	"bytes"
	"testing"
)

func TestParsePlayerRemove(t *testing.T) {
	uuid1 := UUID{0x01}
	uuid2 := UUID{0x02}

	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, 2)
	_ = WriteUUID(buf, uuid1)
	_ = WriteUUID(buf, uuid2)

	parsed, err := ParsePlayerRemove(buf)
	if err != nil {
		t.Fatalf("ParsePlayerRemove failed: %v", err)
	}

	if parsed.PlayerCount != 2 {
		t.Errorf("Expected count 2, got %d", parsed.PlayerCount)
	}
	if len(parsed.Players) != 2 {
		t.Errorf("Expected 2 players, got %d", len(parsed.Players))
	}
	if parsed.Players[0] != uuid1 || parsed.Players[1] != uuid2 {
		t.Errorf("UUID mismatch")
	}
}
