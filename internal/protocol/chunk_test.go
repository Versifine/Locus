package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParseLevelChunkWithLight(t *testing.T) {
	chunkData := new(bytes.Buffer)
	for section := 0; section < ChunkSectionCount; section++ {
		blockStateID := int32(1000 + section)
		if err := writeInt16(chunkData, int16(4096)); err != nil {
			t.Fatalf("write block count failed: %v", err)
		}

		// Block states: single-value paletted container.
		_ = WriteByte(chunkData, 0)
		_ = WriteVarint(chunkData, blockStateID)

		// Biomes: single-value paletted container (ignored by parser).
		_ = WriteByte(chunkData, 0)
		_ = WriteVarint(chunkData, 0)
	}

	payload := new(bytes.Buffer)
	_ = WriteInt32(payload, 12)
	_ = WriteInt32(payload, -34)
	_ = WriteByte(payload, TagEnd) // Heightmaps NBT (skipped)
	_ = WriteVarint(payload, int32(chunkData.Len()))
	_, _ = payload.Write(chunkData.Bytes())

	// Block entities count + light data arrays (all empty in this test).
	_ = WriteVarint(payload, 0)
	for i := 0; i < 6; i++ {
		_ = WriteVarint(payload, 0)
	}

	got, err := ParseLevelChunkWithLight(bytes.NewReader(payload.Bytes()))
	if err != nil {
		t.Fatalf("ParseLevelChunkWithLight failed: %v", err)
	}

	if got.ChunkX != 12 || got.ChunkZ != -34 {
		t.Fatalf("unexpected chunk coords: got (%d, %d), want (12, -34)", got.ChunkX, got.ChunkZ)
	}
	if got.BlockEntityCount != 0 {
		t.Fatalf("unexpected block entity count: got %d, want 0", got.BlockEntityCount)
	}
	if len(got.Sections) != ChunkSectionCount {
		t.Fatalf("unexpected section count: got %d, want %d", len(got.Sections), ChunkSectionCount)
	}

	for i, section := range got.Sections {
		wantState := int32(1000 + i)
		if section.BlockCount != 4096 {
			t.Fatalf("section %d block count = %d, want 4096", i, section.BlockCount)
		}
		if len(section.BlockStates) != BlockStatesPerSection {
			t.Fatalf("section %d block state len = %d, want %d", i, len(section.BlockStates), BlockStatesPerSection)
		}
		for idx, state := range section.BlockStates {
			if state != wantState {
				t.Fatalf("section %d state[%d] = %d, want %d", i, idx, state, wantState)
			}
		}
	}
}

func writeInt16(w *bytes.Buffer, v int16) error {
	return binary.Write(w, binary.BigEndian, v)
}
