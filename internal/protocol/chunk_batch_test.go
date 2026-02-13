package protocol

import (
	"bytes"
	"testing"
)

func TestParseChunkBatchStart(t *testing.T) {
	got, err := ParseChunkBatchStart(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("ParseChunkBatchStart failed: %v", err)
	}
	if got == nil {
		t.Fatalf("expected non-nil ChunkBatchStart")
	}
}

func TestParseChunkBatchFinished(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, 128)

	got, err := ParseChunkBatchFinished(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ParseChunkBatchFinished failed: %v", err)
	}
	if got.BatchSize != 128 {
		t.Fatalf("unexpected batch size: got %d, want 128", got.BatchSize)
	}
}

func TestCreateChunkBatchReceivedPacket(t *testing.T) {
	packet := CreateChunkBatchReceivedPacket(20.0)
	if packet.ID != C2SChunkBatchReceived {
		t.Fatalf("unexpected packet id: got %d, want %d", packet.ID, C2SChunkBatchReceived)
	}

	r := bytes.NewReader(packet.Payload)
	v, err := ReadFloat(r)
	if err != nil {
		t.Fatalf("ReadFloat failed: %v", err)
	}
	if v != 20.0 {
		t.Fatalf("unexpected chunksPerTick: got %f, want 20.0", v)
	}
}
