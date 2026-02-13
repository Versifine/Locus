package protocol

import (
	"bytes"
	"testing"
)

func TestParsePalettedContainerSingleValue(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteByte(buf, 0)      // bitsPerEntry
	_ = WriteVarint(buf, 1234) // single state ID
	_ = WriteVarint(buf, 0)    // data array length

	got, err := ParsePalettedContainer(buf, 32)
	if err != nil {
		t.Fatalf("ParsePalettedContainer failed: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("unexpected result length: got %d, want 32", len(got))
	}
	for i, v := range got {
		if v != 1234 {
			t.Fatalf("entry %d = %d, want 1234", i, v)
		}
	}
}

func TestParsePalettedContainerIndirectPalette(t *testing.T) {
	palette := []int32{5, 10, 20, 40, 80, 160}
	indices := make([]uint32, 64)
	expected := make([]int32, len(indices))
	for i := range indices {
		indices[i] = uint32(i % len(palette))
		expected[i] = palette[i%len(palette)]
	}

	buf := new(bytes.Buffer)
	_ = WriteByte(buf, 5) // indirect
	_ = WriteVarint(buf, int32(len(palette)))
	for _, p := range palette {
		_ = WriteVarint(buf, p)
	}
	packed := packCompacted(indices, 5)
	_ = WriteVarint(buf, int32(len(packed)))
	for _, v := range packed {
		_ = WriteInt64(buf, int64(v))
	}

	got, err := ParsePalettedContainer(buf, len(indices))
	if err != nil {
		t.Fatalf("ParsePalettedContainer failed: %v", err)
	}
	if len(got) != len(expected) {
		t.Fatalf("unexpected result length: got %d, want %d", len(got), len(expected))
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("entry %d = %d, want %d", i, got[i], expected[i])
		}
	}
}

func TestParsePalettedContainerDirectPalette(t *testing.T) {
	values := []uint32{
		1, 2, 3, 4096, 8191, 77, 0, 15,
		4, 5, 6, 7, 8, 9, 10, 2048,
	}
	expected := make([]int32, len(values))
	for i, v := range values {
		expected[i] = int32(v)
	}

	buf := new(bytes.Buffer)
	_ = WriteByte(buf, 13) // direct
	packed := packCompacted(values, 13)
	_ = WriteVarint(buf, int32(len(packed)))
	for _, v := range packed {
		_ = WriteInt64(buf, int64(v))
	}

	got, err := ParsePalettedContainer(buf, len(values))
	if err != nil {
		t.Fatalf("ParsePalettedContainer failed: %v", err)
	}
	if len(got) != len(expected) {
		t.Fatalf("unexpected result length: got %d, want %d", len(got), len(expected))
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("entry %d = %d, want %d", i, got[i], expected[i])
		}
	}
}

func TestParsePalettedContainerInvalidPaletteIndex(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteByte(buf, 4)
	_ = WriteVarint(buf, 2) // palette length
	_ = WriteVarint(buf, 100)
	_ = WriteVarint(buf, 200)

	indices := []uint32{0, 1, 2, 0}
	packed := packCompacted(indices, 4)
	_ = WriteVarint(buf, int32(len(packed)))
	for _, v := range packed {
		_ = WriteInt64(buf, int64(v))
	}

	_, err := ParsePalettedContainer(buf, len(indices))
	if err == nil {
		t.Fatal("expected error for out-of-range palette index, got nil")
	}
}

func TestParsePalettedContainerIndirectSinglePaletteNoDataArray(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteByte(buf, 4)    // indirect
	_ = WriteVarint(buf, 1)  // palette length
	_ = WriteVarint(buf, 42) // only palette value
	_ = WriteVarint(buf, 0)  // no packed data

	got, err := ParsePalettedContainer(buf, 128)
	if err != nil {
		t.Fatalf("ParsePalettedContainer failed: %v", err)
	}
	if len(got) != 128 {
		t.Fatalf("unexpected result length: got %d, want 128", len(got))
	}
	for i, v := range got {
		if v != 42 {
			t.Fatalf("entry %d = %d, want 42", i, v)
		}
	}
}

func TestParsePalettedContainerIndirectMultiPaletteNoDataArray(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteByte(buf, 4) // indirect
	_ = WriteVarint(buf, 3)
	_ = WriteVarint(buf, 99)
	_ = WriteVarint(buf, 100)
	_ = WriteVarint(buf, 101)
	_ = WriteVarint(buf, 0) // no packed data => all index 0

	got, err := ParsePalettedContainer(buf, 32)
	if err != nil {
		t.Fatalf("ParsePalettedContainer failed: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("unexpected result length: got %d, want 32", len(got))
	}
	for i, v := range got {
		if v != 99 {
			t.Fatalf("entry %d = %d, want 99", i, v)
		}
	}
}

func TestParsePalettedContainerBiomeDirectThreshold(t *testing.T) {
	// For biomes, bitsPerEntry=4 should be treated as direct palette (no local palette).
	values := make([]uint32, 64)
	for i := range values {
		values[i] = uint32(i % 16)
	}

	buf := new(bytes.Buffer)
	_ = WriteByte(buf, 4)
	packed := packCompacted(values, 4)
	_ = WriteVarint(buf, int32(len(packed)))
	for _, v := range packed {
		_ = WriteInt64(buf, int64(v))
	}

	got, err := parsePalettedContainer(buf, 64, maxBiomePaletteBits)
	if err != nil {
		t.Fatalf("parsePalettedContainer failed: %v", err)
	}
	if len(got) != 64 {
		t.Fatalf("unexpected result length: got %d, want 64", len(got))
	}
	for i, v := range got {
		if v != int32(values[i]) {
			t.Fatalf("entry %d = %d, want %d", i, v, values[i])
		}
	}
}

func packCompacted(values []uint32, bitsPerEntry int) []uint64 {
	if len(values) == 0 {
		return []uint64{}
	}

	dataLen := (len(values)*bitsPerEntry + 63) / 64
	data := make([]uint64, dataLen)

	var mask uint64
	if bitsPerEntry >= 64 {
		mask = ^uint64(0)
	} else {
		mask = (uint64(1) << bitsPerEntry) - 1
	}

	for i, value := range values {
		v := uint64(value) & mask
		bitIndex := i * bitsPerEntry
		longIndex := bitIndex / 64
		bitOffset := bitIndex % 64

		data[longIndex] |= v << bitOffset
		if bitOffset+bitsPerEntry > 64 {
			data[longIndex+1] |= v >> (64 - bitOffset)
		}
	}

	return data
}
