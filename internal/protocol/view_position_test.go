package protocol

import (
	"bytes"
	"testing"
)

func TestParseUpdateViewPosition(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, -12)
	_ = WriteVarint(buf, 34)

	got, err := ParseUpdateViewPosition(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ParseUpdateViewPosition failed: %v", err)
	}
	if got.ChunkX != -12 || got.ChunkZ != 34 {
		t.Fatalf("unexpected view position: got (%d,%d), want (-12,34)", got.ChunkX, got.ChunkZ)
	}
}
