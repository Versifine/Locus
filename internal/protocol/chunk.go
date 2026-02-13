package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

const (
	ChunkSectionCount     = 24
	BlockStatesPerSection = 16 * 16 * 16
	BiomesPerSection      = 4 * 4 * 4
)

// ChunkSection contains expanded block-state IDs for a 16x16x16 section.
type ChunkSection struct {
	BlockCount  int16
	BlockStates []int32
}

// LevelChunkWithLight is the S2C map_chunk packet payload (protocol 774).
type LevelChunkWithLight struct {
	ChunkX           int32
	ChunkZ           int32
	Heightmaps       []Heightmap
	ChunkData        []byte
	Sections         []ChunkSection
	SectionCount     int
	HasBiomeData     bool
	BlockEntityCount int32
}

type Heightmap struct {
	Type int32
	Data []int64
}

// UnloadChunk is the S2C unload_chunk packet payload (protocol 774).
// NOTE: protocol field order is chunkZ first, then chunkX.
type UnloadChunk struct {
	ChunkX int32
	ChunkZ int32
}

func ParseLevelChunkWithLight(r io.Reader) (*LevelChunkWithLight, error) {
	chunkX, err := ReadInt32(r)
	if err != nil {
		return nil, err
	}
	chunkZ, err := ReadInt32(r)
	if err != nil {
		return nil, err
	}

	heightmaps, err := readHeightmaps(r)
	if err != nil {
		return nil, err
	}

	chunkData, err := readVarIntByteArray(r)
	if err != nil {
		return nil, err
	}

	sections, sectionCount, hasBiomeData, err := ParseChunkSectionsAuto(chunkData)
	if err != nil {
		return nil, err
	}

	blockEntityCount, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	if blockEntityCount < 0 {
		return nil, fmt.Errorf("invalid block entity count: %d", blockEntityCount)
	}
	for i := int32(0); i < blockEntityCount; i++ {
		if err := skipChunkBlockEntity(r); err != nil {
			return nil, fmt.Errorf("failed to parse block entity %d: %w", i, err)
		}
	}

	if err := skipLightData(r); err != nil {
		return nil, err
	}

	return &LevelChunkWithLight{
		ChunkX:           chunkX,
		ChunkZ:           chunkZ,
		Heightmaps:       heightmaps,
		ChunkData:        chunkData,
		Sections:         sections,
		SectionCount:     sectionCount,
		HasBiomeData:     hasBiomeData,
		BlockEntityCount: blockEntityCount,
	}, nil
}

type chunkSectionParseAttempt struct {
	SectionCount int
	WithBiomes   bool
	Encoding     chunkPalettedEncoding
	Err          error
}

type chunkPalettedEncoding int

const (
	// 1.21.11 captures show section paletted containers without explicit data-array length.
	// Data long count is derived from padded packing rules.
	chunkPalettedNoLengthPadded chunkPalettedEncoding = iota
	// Legacy/older chunk captures with explicit data-array length VarInt.
	chunkPalettedLengthPrefixed
)

func (e chunkPalettedEncoding) String() string {
	switch e {
	case chunkPalettedNoLengthPadded:
		return "no_len_padded"
	case chunkPalettedLengthPrefixed:
		return "len_prefixed"
	default:
		return "unknown"
	}
}

var defaultChunkSectionCandidates = []int{
	ChunkSectionCount, // 24 (overworld-like)
	16,                // nether/end-like
	20,
	12,
	8,
	28,
	32,
}

func ParseChunkSectionsAuto(chunkData []byte) ([]ChunkSection, int, bool, error) {
	encodings := []chunkPalettedEncoding{
		chunkPalettedNoLengthPadded,
		chunkPalettedLengthPrefixed,
	}
	attempts := make([]chunkSectionParseAttempt, 0, len(defaultChunkSectionCandidates)*len(encodings)*2)

	for _, sectionCount := range defaultChunkSectionCandidates {
		for _, encoding := range encodings {
			for _, withBiomes := range []bool{true, false} {
				sections, err := parseChunkSections(chunkData, sectionCount, withBiomes, encoding)
				if err == nil {
					return sections, sectionCount, withBiomes, nil
				}
				attempts = append(attempts, chunkSectionParseAttempt{
					SectionCount: sectionCount,
					WithBiomes:   withBiomes,
					Encoding:     encoding,
					Err:          err,
				})
			}
		}
	}

	msg := "chunk sections auto-parse failed: "
	for i, a := range attempts {
		if i > 0 {
			msg += "; "
		}
		msg += fmt.Sprintf(
			"count=%d withBiomes=%t encoding=%s err=%v",
			a.SectionCount,
			a.WithBiomes,
			a.Encoding.String(),
			a.Err,
		)
	}
	return nil, 0, false, errors.New(msg)
}

func ParseChunkSections(chunkData []byte, sectionCount int) ([]ChunkSection, error) {
	encodings := []chunkPalettedEncoding{
		chunkPalettedNoLengthPadded,
		chunkPalettedLengthPrefixed,
	}

	attemptMsg := ""
	for _, encoding := range encodings {
		sections, err := parseChunkSections(chunkData, sectionCount, true, encoding)
		if err == nil {
			return sections, nil
		}
		if attemptMsg != "" {
			attemptMsg += "; "
		}
		attemptMsg += fmt.Sprintf("encoding=%s withBiomes=true err=%v", encoding.String(), err)

		sectionsNoBiomes, errNoBiomes := parseChunkSections(chunkData, sectionCount, false, encoding)
		if errNoBiomes == nil {
			return sectionsNoBiomes, nil
		}
		attemptMsg += fmt.Sprintf("; encoding=%s withBiomes=false err=%v", encoding.String(), errNoBiomes)
	}

	return nil, fmt.Errorf("chunk sections parse failed: %s", attemptMsg)
}

func parseChunkSections(
	chunkData []byte,
	sectionCount int,
	withBiomes bool,
	encoding chunkPalettedEncoding,
) ([]ChunkSection, error) {
	if sectionCount <= 0 {
		return nil, fmt.Errorf("invalid section count: %d", sectionCount)
	}

	reader := bytes.NewReader(chunkData)
	sections := make([]ChunkSection, sectionCount)
	for i := 0; i < sectionCount; i++ {
		blockCount, err := ReadInt16(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read section %d block count: %w", i, err)
		}

		blockStates, err := parseChunkPalettedContainer(reader, BlockStatesPerSection, maxIndirectPaletteBits, encoding)
		if err != nil {
			return nil, fmt.Errorf("failed to read section %d block states: %w", i, err)
		}

		if withBiomes {
			if _, err := parseChunkPalettedContainer(reader, BiomesPerSection, maxBiomePaletteBits, encoding); err != nil {
				return nil, fmt.Errorf("failed to read section %d biomes: %w", i, err)
			}
		}

		sections[i] = ChunkSection{
			BlockCount:  blockCount,
			BlockStates: blockStates,
		}
	}

	if reader.Len() != 0 {
		return nil, fmt.Errorf("chunk data has %d unread bytes", reader.Len())
	}
	return sections, nil
}

func parseChunkPalettedContainer(
	r *bytes.Reader,
	entryCount int,
	indirectMaxBits int,
	encoding chunkPalettedEncoding,
) ([]int32, error) {
	switch encoding {
	case chunkPalettedNoLengthPadded:
		return parsePalettedContainerNoLengthPadded(r, entryCount, indirectMaxBits)
	case chunkPalettedLengthPrefixed:
		return parsePalettedContainer(r, entryCount, indirectMaxBits)
	default:
		return nil, fmt.Errorf("unknown chunk paletted encoding: %d", encoding)
	}
}

// parsePalettedContainerNoLengthPadded parses 1.21.11-like paletted containers:
// - bitsPerEntry == 0: single value only (no data-array length field)
// - bitsPerEntry > 0: no explicit data-array length; long count is implied by padded packing.
func parsePalettedContainerNoLengthPadded(r io.Reader, entryCount int, indirectMaxBits int) ([]int32, error) {
	if entryCount < 0 {
		return nil, fmt.Errorf("invalid entry count: %d", entryCount)
	}
	if entryCount == 0 {
		return []int32{}, nil
	}
	if indirectMaxBits <= 0 || indirectMaxBits > maxIndirectPaletteBits {
		return nil, fmt.Errorf("invalid indirect max bits: %d", indirectMaxBits)
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

	isIndirect := bitsPerEntry <= indirectMaxBits
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

	dataArrayLen := expectedPaddedLen(entryCount, bitsPerEntry)
	if dataArrayLen <= 0 {
		return nil, fmt.Errorf(
			"invalid implied data array length for entryCount=%d bitsPerEntry=%d: %d",
			entryCount,
			bitsPerEntry,
			dataArrayLen,
		)
	}

	packed := make([]uint64, dataArrayLen)
	for i := 0; i < dataArrayLen; i++ {
		v, err := ReadInt64(r)
		if err != nil {
			return nil, err
		}
		packed[i] = uint64(v)
	}

	values, err := unpackPadded(packed, bitsPerEntry, entryCount)
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
			// Some captures carry empty indirect palettes. Treat index 0 as global 0
			// so parsing can continue without corrupting stream alignment.
			if len(palette) == 0 && paletteIndex == 0 {
				expanded[i] = 0
				continue
			}
			return nil, fmt.Errorf("palette index out of range: %d (palette len: %d)", paletteIndex, len(palette))
		}
		expanded[i] = palette[paletteIndex]
	}
	return expanded, nil
}

func ParseUnloadChunk(r io.Reader) (*UnloadChunk, error) {
	chunkZ, err := ReadInt32(r)
	if err != nil {
		return nil, err
	}
	chunkX, err := ReadInt32(r)
	if err != nil {
		return nil, err
	}
	return &UnloadChunk{
		ChunkX: chunkX,
		ChunkZ: chunkZ,
	}, nil
}

func readHeightmaps(r io.Reader) ([]Heightmap, error) {
	count, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	if count < 0 {
		return nil, fmt.Errorf("invalid heightmap count: %d", count)
	}

	heightmaps := make([]Heightmap, count)
	for i := int32(0); i < count; i++ {
		hType, err := ReadVarint(r)
		if err != nil {
			return nil, err
		}

		dataCount, err := ReadVarint(r)
		if err != nil {
			return nil, err
		}
		if dataCount < 0 {
			return nil, fmt.Errorf("invalid heightmap data length: %d", dataCount)
		}

		data := make([]int64, dataCount)
		for j := int32(0); j < dataCount; j++ {
			v, err := ReadInt64(r)
			if err != nil {
				return nil, err
			}
			data[j] = v
		}

		heightmaps[i] = Heightmap{
			Type: hType,
			Data: data,
		}
	}

	return heightmaps, nil
}

func readVarIntByteArray(r io.Reader) ([]byte, error) {
	length, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("invalid byte array length: %d", length)
	}
	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func skipChunkBlockEntity(r io.Reader) error {
	if _, err := ReadByte(r); err != nil { // packed x/z nibble
		return err
	}
	if _, err := ReadInt16(r); err != nil {
		return err
	}
	if _, err := ReadVarint(r); err != nil {
		return err
	}
	_, err := ReadAnonymousNBT(r)
	return err
}

func skipLightData(r io.Reader) error {
	// skyLightMask, blockLightMask, emptySkyLightMask, emptyBlockLightMask
	for i := 0; i < 4; i++ {
		if err := skipInt64Array(r); err != nil {
			return err
		}
	}
	// skyLight, blockLight
	for i := 0; i < 2; i++ {
		if err := skipByteArrayArray(r); err != nil {
			return err
		}
	}
	return nil
}

func skipInt64Array(r io.Reader) error {
	count, err := ReadVarint(r)
	if err != nil {
		return err
	}
	if count < 0 {
		return fmt.Errorf("invalid int64 array length: %d", count)
	}
	for i := int32(0); i < count; i++ {
		if _, err := ReadInt64(r); err != nil {
			return err
		}
	}
	return nil
}

func skipByteArrayArray(r io.Reader) error {
	count, err := ReadVarint(r)
	if err != nil {
		return err
	}
	if count < 0 {
		return fmt.Errorf("invalid byte-array array length: %d", count)
	}
	for i := int32(0); i < count; i++ {
		if _, err := readVarIntByteArray(r); err != nil {
			return err
		}
	}
	return nil
}
