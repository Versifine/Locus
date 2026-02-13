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
		_ = WriteVarint(chunkData, 0)

		// Biomes: single-value paletted container (ignored by parser).
		_ = WriteByte(chunkData, 0)
		_ = WriteVarint(chunkData, 0)
		_ = WriteVarint(chunkData, 0)
	}

	payload := new(bytes.Buffer)
	_ = WriteInt32(payload, 12)
	_ = WriteInt32(payload, -34)
	_ = WriteVarint(payload, 0) // Heightmaps array (empty in this test)
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
	if len(got.Heightmaps) != 0 {
		t.Fatalf("unexpected heightmap count: got %d, want 0", len(got.Heightmaps))
	}
	if got.BlockEntityCount != 0 {
		t.Fatalf("unexpected block entity count: got %d, want 0", got.BlockEntityCount)
	}
	if got.SectionCount != ChunkSectionCount {
		t.Fatalf("unexpected parsed section count: got %d, want %d", got.SectionCount, ChunkSectionCount)
	}
	if !got.HasBiomeData {
		t.Fatalf("expected hasBiomeData=true for test payload")
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

func TestParseChunkSectionsWithoutBiomesFallback(t *testing.T) {
	chunkData := new(bytes.Buffer)
	for section := 0; section < ChunkSectionCount; section++ {
		blockStateID := int32(2000 + section)
		if err := writeInt16(chunkData, int16(4096)); err != nil {
			t.Fatalf("write block count failed: %v", err)
		}
		_ = WriteByte(chunkData, 0) // block states single-value
		_ = WriteVarint(chunkData, blockStateID)
		_ = WriteVarint(chunkData, 0) // data array length
	}

	sections, err := ParseChunkSections(chunkData.Bytes(), ChunkSectionCount)
	if err != nil {
		t.Fatalf("ParseChunkSections failed: %v", err)
	}
	if len(sections) != ChunkSectionCount {
		t.Fatalf("len(sections) = %d, want %d", len(sections), ChunkSectionCount)
	}
	for i, section := range sections {
		want := int32(2000 + i)
		if len(section.BlockStates) != BlockStatesPerSection {
			t.Fatalf("section %d len = %d, want %d", i, len(section.BlockStates), BlockStatesPerSection)
		}
		if section.BlockStates[0] != want || section.BlockStates[len(section.BlockStates)-1] != want {
			t.Fatalf("section %d decoded state mismatch, want %d", i, want)
		}
	}
}

func TestParseChunkSectionsAuto16Sections(t *testing.T) {
	chunkData := new(bytes.Buffer)
	const sectionCount16 = 16

	for section := 0; section < sectionCount16; section++ {
		blockStateID := int32(3000 + section)
		if err := writeInt16(chunkData, int16(4096)); err != nil {
			t.Fatalf("write block count failed: %v", err)
		}
		_ = WriteByte(chunkData, 0) // block states single-value
		_ = WriteVarint(chunkData, blockStateID)
		_ = WriteVarint(chunkData, 0) // data array length
	}

	sections, parsedCount, hasBiomeData, err := ParseChunkSectionsAuto(chunkData.Bytes())
	if err != nil {
		t.Fatalf("ParseChunkSectionsAuto failed: %v", err)
	}
	if parsedCount != sectionCount16 {
		t.Fatalf("parsedCount = %d, want %d", parsedCount, sectionCount16)
	}
	if hasBiomeData {
		t.Fatalf("expected hasBiomeData=false for 16-section no-biome payload")
	}
	if len(sections) != sectionCount16 {
		t.Fatalf("len(sections) = %d, want %d", len(sections), sectionCount16)
	}
}

func TestParseChunkSectionsAuto16SectionsWithBiomes(t *testing.T) {
	chunkData := new(bytes.Buffer)
	const sectionCount16 = 16

	for section := 0; section < sectionCount16; section++ {
		blockStateID := int32(4000 + section)
		if err := writeInt16(chunkData, int16(4096)); err != nil {
			t.Fatalf("write block count failed: %v", err)
		}
		_ = WriteByte(chunkData, 0) // block states single-value
		_ = WriteVarint(chunkData, blockStateID)
		_ = WriteVarint(chunkData, 0)

		_ = WriteByte(chunkData, 0) // biomes single-value
		_ = WriteVarint(chunkData, int32(3))
		_ = WriteVarint(chunkData, 0)
	}

	sections, parsedCount, hasBiomeData, err := ParseChunkSectionsAuto(chunkData.Bytes())
	if err != nil {
		t.Fatalf("ParseChunkSectionsAuto failed: %v", err)
	}
	if parsedCount != sectionCount16 {
		t.Fatalf("parsedCount = %d, want %d", parsedCount, sectionCount16)
	}
	if !hasBiomeData {
		t.Fatalf("expected hasBiomeData=true for 16-section payload with biomes")
	}
	if len(sections) != sectionCount16 {
		t.Fatalf("len(sections) = %d, want %d", len(sections), sectionCount16)
	}
}

func TestParseChunkSectionsAuto24SectionsWithoutBiomes(t *testing.T) {
	chunkData := new(bytes.Buffer)

	for section := 0; section < ChunkSectionCount; section++ {
		blockStateID := int32(5000 + section)
		if err := writeInt16(chunkData, int16(4096)); err != nil {
			t.Fatalf("write block count failed: %v", err)
		}
		_ = WriteByte(chunkData, 0) // block states single-value
		_ = WriteVarint(chunkData, blockStateID)
		_ = WriteVarint(chunkData, 0)
	}

	sections, parsedCount, hasBiomeData, err := ParseChunkSectionsAuto(chunkData.Bytes())
	if err != nil {
		t.Fatalf("ParseChunkSectionsAuto failed: %v", err)
	}
	if parsedCount != ChunkSectionCount {
		t.Fatalf("parsedCount = %d, want %d", parsedCount, ChunkSectionCount)
	}
	if hasBiomeData {
		t.Fatalf("expected hasBiomeData=false for 24-section payload without biomes")
	}
	if len(sections) != ChunkSectionCount {
		t.Fatalf("len(sections) = %d, want %d", len(sections), ChunkSectionCount)
	}
}

func TestParseChunkSectionsAutoNoLengthSingleValueFormat(t *testing.T) {
	chunkData := new(bytes.Buffer)
	for section := 0; section < ChunkSectionCount; section++ {
		if err := writeInt16(chunkData, int16(0)); err != nil {
			t.Fatalf("write block count failed: %v", err)
		}
		// 1.21.11-like single-value container: bits=0 + value, no data-array length.
		_ = WriteByte(chunkData, 0) // block states bitsPerEntry
		_ = WriteVarint(chunkData, 0)
		_ = WriteByte(chunkData, 0) // biomes bitsPerEntry
		_ = WriteVarint(chunkData, 0)
	}

	sections, parsedCount, hasBiomeData, err := ParseChunkSectionsAuto(chunkData.Bytes())
	if err != nil {
		t.Fatalf("ParseChunkSectionsAuto failed: %v", err)
	}
	if parsedCount != ChunkSectionCount {
		t.Fatalf("parsedCount = %d, want %d", parsedCount, ChunkSectionCount)
	}
	if !hasBiomeData {
		t.Fatalf("expected hasBiomeData=true for no-length single-value format")
	}
	if len(sections) != ChunkSectionCount {
		t.Fatalf("len(sections) = %d, want %d", len(sections), ChunkSectionCount)
	}
	for i, section := range sections {
		if section.BlockCount != 0 {
			t.Fatalf("section %d block count = %d, want 0", i, section.BlockCount)
		}
		if len(section.BlockStates) != BlockStatesPerSection {
			t.Fatalf("section %d block state len = %d, want %d", i, len(section.BlockStates), BlockStatesPerSection)
		}
		if section.BlockStates[0] != 0 || section.BlockStates[len(section.BlockStates)-1] != 0 {
			t.Fatalf("section %d block states should be all zero", i)
		}
	}
}

func writeInt16(w *bytes.Buffer, v int16) error {
	return binary.Write(w, binary.BigEndian, v)
}

func TestParseUnloadChunk(t *testing.T) {
	payload := new(bytes.Buffer)
	// Protocol order: chunkZ, chunkX
	_ = WriteInt32(payload, -7)
	_ = WriteInt32(payload, 12)

	got, err := ParseUnloadChunk(bytes.NewReader(payload.Bytes()))
	if err != nil {
		t.Fatalf("ParseUnloadChunk failed: %v", err)
	}

	if got.ChunkX != 12 || got.ChunkZ != -7 {
		t.Fatalf("unexpected coords: got (x=%d,z=%d), want (x=12,z=-7)", got.ChunkX, got.ChunkZ)
	}
}
