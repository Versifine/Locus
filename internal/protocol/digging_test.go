package protocol

import (
	"bytes"
	"testing"
)

func TestParseAcknowledgePlayerDigging(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, 42)

	got, err := ParseAcknowledgePlayerDigging(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ParseAcknowledgePlayerDigging failed: %v", err)
	}
	if got.SequenceID != 42 {
		t.Fatalf("unexpected sequence id: got %d, want 42", got.SequenceID)
	}
}

func TestCreateBlockDigPacket(t *testing.T) {
	packet := CreateBlockDigPacket(0, BlockPos{X: -3, Y: 64, Z: 9}, 1, 77)
	if packet.ID != C2SBlockDig {
		t.Fatalf("unexpected packet id: got %d, want %d", packet.ID, C2SBlockDig)
	}

	r := bytes.NewReader(packet.Payload)
	status, err := ReadVarint(r)
	if err != nil {
		t.Fatalf("read status failed: %v", err)
	}
	rawPos, err := ReadInt64(r)
	if err != nil {
		t.Fatalf("read packed position failed: %v", err)
	}
	face, err := ReadByte(r)
	if err != nil {
		t.Fatalf("read face failed: %v", err)
	}
	sequence, err := ReadVarint(r)
	if err != nil {
		t.Fatalf("read sequence failed: %v", err)
	}

	x, y, z := decodePackedPosition(rawPos)
	if status != 0 || x != -3 || y != 64 || z != 9 || int8(face) != 1 || sequence != 77 {
		t.Fatalf(
			"unexpected block dig payload: status=%d pos=(%d,%d,%d) face=%d sequence=%d",
			status,
			x,
			y,
			z,
			int8(face),
			sequence,
		)
	}
}

func TestParseAcknowledgePlayerDiggingInvalidSequence(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, -1)

	if _, err := ParseAcknowledgePlayerDigging(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatalf("expected error for invalid sequence id")
	}
}

func decodePackedPosition(raw int64) (int32, int32, int32) {
	v := uint64(raw)
	x := signExtendInt32(int64((v>>38)&0x3FFFFFF), 26)
	z := signExtendInt32(int64((v>>12)&0x3FFFFFF), 26)
	y := signExtendInt32(int64(v&0xFFF), 12)
	return x, y, z
}
