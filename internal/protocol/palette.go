package protocol

import (
	"fmt"
	"io"
)

const (
	maxIndirectPaletteBits = 8
	maxPaletteBitsPerEntry = 32
)

// ParsePalettedContainer parses a paletted container and expands it to entryCount block-state IDs.
func ParsePalettedContainer(r io.Reader, entryCount int) ([]int32, error) {
	if entryCount < 0 {
		return nil, fmt.Errorf("invalid entry count: %d", entryCount)
	}
	if entryCount == 0 {
		return []int32{}, nil
	}

	bitsByte, err := ReadByte(r)
	if err != nil {
		return nil, err
	}
	bitsPerEntry := int(bitsByte)
	if bitsPerEntry > maxPaletteBitsPerEntry {
		return nil, fmt.Errorf("bits per entry too large: %d", bitsPerEntry)
	}

	if bitsPerEntry == 0 {
		value, err := ReadVarint(r)
		if err != nil {
			return nil, err
		}
		expanded := make([]int32, entryCount)
		for i := range expanded {
			expanded[i] = value
		}
		return expanded, nil
	}

	isIndirect := bitsPerEntry <= maxIndirectPaletteBits
	var palette []int32
	if isIndirect {
		paletteLen, err := ReadVarint(r)
		if err != nil {
			return nil, err
		}
		if paletteLen < 0 {
			return nil, fmt.Errorf("invalid palette length: %d", paletteLen)
		}
		palette = make([]int32, paletteLen)
		for i := int32(0); i < paletteLen; i++ {
			entry, err := ReadVarint(r)
			if err != nil {
				return nil, err
			}
			palette[i] = entry
		}
	}

	dataArrayLen, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	if dataArrayLen < 0 {
		return nil, fmt.Errorf("invalid data array length: %d", dataArrayLen)
	}

	packed := make([]uint64, dataArrayLen)
	for i := int32(0); i < dataArrayLen; i++ {
		v, err := ReadInt64(r)
		if err != nil {
			return nil, err
		}
		packed[i] = uint64(v)
	}

	values, err := unpackPalettedValues(packed, bitsPerEntry, entryCount)
	if err != nil {
		return nil, err
	}

	expanded := make([]int32, entryCount)
	if !isIndirect {
		for i := range values {
			expanded[i] = int32(values[i])
		}
		return expanded, nil
	}

	for i, paletteIndex := range values {
		if int(paletteIndex) >= len(palette) {
			return nil, fmt.Errorf("palette index out of range: %d (palette len: %d)", paletteIndex, len(palette))
		}
		expanded[i] = palette[paletteIndex]
	}
	return expanded, nil
}

func unpackPalettedValues(data []uint64, bitsPerEntry, entryCount int) ([]int, error) {
	if bitsPerEntry <= 0 || bitsPerEntry > 64 {
		return nil, fmt.Errorf("invalid bits per entry: %d", bitsPerEntry)
	}
	if entryCount < 0 {
		return nil, fmt.Errorf("invalid entry count: %d", entryCount)
	}
	if entryCount == 0 {
		return []int{}, nil
	}

	compactedLen := expectedCompactedLen(entryCount, bitsPerEntry)
	paddedLen := expectedPaddedLen(entryCount, bitsPerEntry)

	switch len(data) {
	case compactedLen:
		return unpackCompacted(data, bitsPerEntry, entryCount)
	case paddedLen:
		return unpackPadded(data, bitsPerEntry, entryCount)
	default:
		return nil, fmt.Errorf(
			"invalid data array length: got %d, expected %d (compacted) or %d (padded)",
			len(data),
			compactedLen,
			paddedLen,
		)
	}
}

func unpackCompacted(data []uint64, bitsPerEntry, entryCount int) ([]int, error) {
	mask := valueMask(bitsPerEntry)
	values := make([]int, entryCount)

	for i := 0; i < entryCount; i++ {
		bitIndex := i * bitsPerEntry
		longIndex := bitIndex / 64
		bitOffset := bitIndex % 64
		if longIndex >= len(data) {
			return nil, fmt.Errorf("packed data ended early at entry %d", i)
		}

		value := data[longIndex] >> bitOffset
		if bitOffset+bitsPerEntry > 64 {
			if longIndex+1 >= len(data) {
				return nil, fmt.Errorf("packed data ended early at entry %d", i)
			}
			value |= data[longIndex+1] << (64 - bitOffset)
		}
		values[i] = int(value & mask)
	}
	return values, nil
}

func unpackPadded(data []uint64, bitsPerEntry, entryCount int) ([]int, error) {
	valuesPerLong := 64 / bitsPerEntry
	if valuesPerLong <= 0 {
		return nil, fmt.Errorf("invalid bits per entry for padded format: %d", bitsPerEntry)
	}

	mask := valueMask(bitsPerEntry)
	values := make([]int, entryCount)
	for i := 0; i < entryCount; i++ {
		longIndex := i / valuesPerLong
		if longIndex >= len(data) {
			return nil, fmt.Errorf("packed data ended early at entry %d", i)
		}
		bitOffset := (i % valuesPerLong) * bitsPerEntry
		values[i] = int((data[longIndex] >> bitOffset) & mask)
	}
	return values, nil
}

func expectedCompactedLen(entryCount, bitsPerEntry int) int {
	return (entryCount*bitsPerEntry + 63) / 64
}

func expectedPaddedLen(entryCount, bitsPerEntry int) int {
	valuesPerLong := 64 / bitsPerEntry
	if valuesPerLong <= 0 {
		return 0
	}
	return (entryCount + valuesPerLong - 1) / valuesPerLong
}

func valueMask(bitsPerEntry int) uint64 {
	if bitsPerEntry >= 64 {
		return ^uint64(0)
	}
	return (uint64(1) << bitsPerEntry) - 1
}
