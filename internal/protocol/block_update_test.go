package protocol

import (
	"bytes"
	"testing"
)

func TestParseBlockChange(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteInt64(buf, packBlockPosition(4, 73, 9))
	_ = WriteVarint(buf, 1)

	got, err := ParseBlockChange(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ParseBlockChange failed: %v", err)
	}
	if got.X != 4 || got.Y != 73 || got.Z != 9 {
		t.Fatalf("unexpected location: got (%d,%d,%d), want (4,73,9)", got.X, got.Y, got.Z)
	}
	if got.StateID != 1 {
		t.Fatalf("unexpected state id: got %d, want 1", got.StateID)
	}
}

func TestParseMultiBlockChange(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteInt64(buf, packChunkSectionPosition(0, 4, 0))
	_ = WriteVarint(buf, 2)
	// state=1 at local (4,9,9) => global (4,73,9)
	_ = WriteVarint(buf, packMultiBlockRecord(1, 4, 9, 9))
	// state=0 at local (4,9,9) => global (4,73,9)
	_ = WriteVarint(buf, packMultiBlockRecord(0, 4, 9, 9))

	got, err := ParseMultiBlockChange(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ParseMultiBlockChange failed: %v", err)
	}
	if got.ChunkX != 0 || got.ChunkY != 4 || got.ChunkZ != 0 {
		t.Fatalf("unexpected chunk section coords: got (%d,%d,%d), want (0,4,0)", got.ChunkX, got.ChunkY, got.ChunkZ)
	}
	if len(got.Records) != 2 {
		t.Fatalf("unexpected record count: got %d, want 2", len(got.Records))
	}
	if got.Records[0].X != 4 || got.Records[0].Y != 73 || got.Records[0].Z != 9 || got.Records[0].StateID != 1 {
		t.Fatalf("unexpected first record: %+v", got.Records[0])
	}
	if got.Records[1].X != 4 || got.Records[1].Y != 73 || got.Records[1].Z != 9 || got.Records[1].StateID != 0 {
		t.Fatalf("unexpected second record: %+v", got.Records[1])
	}
}

func packBlockPosition(x, y, z int32) int64 {
	ux := uint64(int64(x) & 0x3FFFFFF)
	uy := uint64(int64(y) & 0xFFF)
	uz := uint64(int64(z) & 0x3FFFFFF)
	return int64((ux << 38) | (uz << 12) | uy)
}

func packChunkSectionPosition(chunkX, chunkY, chunkZ int32) int64 {
	ux := uint64(int64(chunkX) & 0x3FFFFF)
	uy := uint64(int64(chunkY) & 0xFFFFF)
	uz := uint64(int64(chunkZ) & 0x3FFFFF)
	return int64((ux << 42) | (uz << 20) | uy)
}

func packMultiBlockRecord(stateID, localX, localY, localZ int32) int32 {
	local := ((localX & 0x0F) << 8) | ((localZ & 0x0F) << 4) | (localY & 0x0F)
	return (stateID << 12) | local
}
