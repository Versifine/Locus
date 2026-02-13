package bot

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Versifine/locus/internal/protocol"
	"github.com/Versifine/locus/internal/world"
)

func normalizeSectionsForBlockStore(parsed []protocol.ChunkSection) ([]world.ChunkSection, error) {
	if len(parsed) == 0 {
		return nil, fmt.Errorf("no parsed sections")
	}
	if len(parsed) > world.ChunkSectionCount {
		return nil, fmt.Errorf("too many parsed sections: %d", len(parsed))
	}

	offset := (world.ChunkSectionCount - len(parsed)) / 2
	if offset < 0 {
		offset = 0
	}

	normalized := make([]world.ChunkSection, world.ChunkSectionCount)
	for i := range normalized {
		normalized[i] = world.ChunkSection{BlockStates: make([]int32, world.BlocksPerSection)}
	}

	for i, section := range parsed {
		target := i + offset
		if target < 0 || target >= len(normalized) {
			return nil, fmt.Errorf("section index out of range after normalize: parsed=%d target=%d", i, target)
		}
		if len(section.BlockStates) != world.BlocksPerSection {
			return nil, fmt.Errorf(
				"invalid parsed section %d block state len: got %d, want %d",
				i,
				len(section.BlockStates),
				world.BlocksPerSection,
			)
		}
		copy(normalized[target].BlockStates, section.BlockStates)
	}

	return normalized, nil
}

type chunkCaptureMeta struct {
	CapturedAt string `json:"captured_at"`
	ParseError string `json:"parse_error"`
	PayloadLen int    `json:"payload_len"`

	HasChunkCoord bool  `json:"has_chunk_coord"`
	ChunkX        int32 `json:"chunk_x,omitempty"`
	ChunkZ        int32 `json:"chunk_z,omitempty"`

	PayloadFile string `json:"payload_file"`
	PrefixHex64 string `json:"prefix_hex_64"`
}

func (b *Bot) captureFailedChunkPayload(payload []byte, parseErr error) {
	if b.chunkCaptureMax <= 0 || b.chunkCaptureDir == "" {
		return
	}

	b.chunkCaptureMu.Lock()
	defer b.chunkCaptureMu.Unlock()

	if b.chunkCaptureCount >= b.chunkCaptureMax {
		return
	}
	b.chunkCaptureCount++
	index := b.chunkCaptureCount

	if err := os.MkdirAll(b.chunkCaptureDir, 0o755); err != nil {
		slog.Warn("Failed to create chunk payload capture dir", "dir", b.chunkCaptureDir, "error", err)
		return
	}

	hasCoord, chunkX, chunkZ := extractChunkCoords(payload)
	ts := time.Now().Format("20060102_150405_000")
	baseName := fmt.Sprintf("%s_%02d_len%d", ts, index, len(payload))
	if hasCoord {
		baseName = fmt.Sprintf("%s_x%d_z%d", baseName, chunkX, chunkZ)
	}

	payloadFile := filepath.Join(b.chunkCaptureDir, baseName+".bin")
	if err := os.WriteFile(payloadFile, payload, 0o644); err != nil {
		slog.Warn("Failed to write chunk payload capture file", "file", payloadFile, "error", err)
		return
	}

	prefixLen := len(payload)
	if prefixLen > 64 {
		prefixLen = 64
	}

	meta := chunkCaptureMeta{
		CapturedAt:    time.Now().Format(time.RFC3339Nano),
		ParseError:    fmt.Sprintf("%v", parseErr),
		PayloadLen:    len(payload),
		HasChunkCoord: hasCoord,
		ChunkX:        chunkX,
		ChunkZ:        chunkZ,
		PayloadFile:   filepath.Base(payloadFile),
		PrefixHex64:   hex.EncodeToString(payload[:prefixLen]),
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		slog.Warn("Failed to encode chunk payload capture meta", "error", err)
		return
	}

	metaFile := filepath.Join(b.chunkCaptureDir, baseName+".json")
	if err := os.WriteFile(metaFile, metaBytes, 0o644); err != nil {
		slog.Warn("Failed to write chunk payload capture meta file", "file", metaFile, "error", err)
		return
	}

	slog.Info(
		"Captured failed chunk payload",
		"index", index,
		"max", b.chunkCaptureMax,
		"payload_file", payloadFile,
		"meta_file", metaFile,
		"parse_error", parseErr,
	)
}

func extractChunkCoords(payload []byte) (bool, int32, int32) {
	if len(payload) < 8 {
		return false, 0, 0
	}
	chunkX := int32(binary.BigEndian.Uint32(payload[0:4]))
	chunkZ := int32(binary.BigEndian.Uint32(payload[4:8]))
	return true, chunkX, chunkZ
}

func (b *Bot) noteChunkLoad() {
	b.chunkStatsMu.Lock()
	defer b.chunkStatsMu.Unlock()
	b.chunkLoadEvents++
	b.maybeLogChunkStatsLocked()
}

func (b *Bot) noteChunkUnload() {
	b.chunkStatsMu.Lock()
	defer b.chunkStatsMu.Unlock()
	b.chunkUnloadEvents++
	b.maybeLogChunkStatsLocked()
}

func (b *Bot) maybeLogChunkStatsLocked() {
	now := time.Now()
	if b.lastChunkStatsLogAt.IsZero() {
		b.lastChunkStatsLogAt = now
	}

	const chunkStatsLogInterval = 2 * time.Second
	const chunkStatsLogBurstThreshold = 32

	totalSinceLast := b.chunkLoadEvents + b.chunkUnloadEvents
	shouldLog := now.Sub(b.lastChunkStatsLogAt) >= chunkStatsLogInterval ||
		totalSinceLast >= chunkStatsLogBurstThreshold
	if !shouldLog {
		return
	}

	loadedChunks := 0
	if b.blockStore != nil {
		loadedChunks = b.blockStore.LoadedChunkCount()
	}

	slog.Info(
		"Chunk sync stats",
		"loaded_chunks", loadedChunks,
		"load_events", b.chunkLoadEvents,
		"unload_events", b.chunkUnloadEvents,
	)

	b.chunkLoadEvents = 0
	b.chunkUnloadEvents = 0
	b.lastChunkStatsLogAt = now
}

func (b *Bot) beginChunkBatch() {
	b.chunkBatchMu.Lock()
	defer b.chunkBatchMu.Unlock()

	if b.chunkBatchActive {
		slog.Warn(
			"Chunk batch start received before previous batch finished",
			"previous_batch_id", b.chunkBatchCurrentID,
			"load_events", b.chunkBatchLoadEvents,
			"unload_events", b.chunkBatchUnloadEvents,
		)
	}

	b.chunkBatchSeq++
	b.chunkBatchCurrentID = b.chunkBatchSeq
	b.chunkBatchActive = true
	b.chunkBatchStartedAt = time.Now()
	b.chunkBatchLoadEvents = 0
	b.chunkBatchUnloadEvents = 0

	slog.Debug("Received chunk batch start", "batch_id", b.chunkBatchCurrentID)
}

func (b *Bot) noteChunkBatchChunkLoad() {
	b.chunkBatchMu.Lock()
	defer b.chunkBatchMu.Unlock()
	if !b.chunkBatchActive {
		return
	}
	b.chunkBatchLoadEvents++
}

func (b *Bot) noteChunkBatchChunkUnload() {
	b.chunkBatchMu.Lock()
	defer b.chunkBatchMu.Unlock()
	if !b.chunkBatchActive {
		return
	}
	b.chunkBatchUnloadEvents++
}

func (b *Bot) finishChunkBatch(batchSize int32) chunkBatchSummary {
	now := time.Now()

	b.chunkBatchMu.Lock()
	summary := chunkBatchSummary{
		BatchSize:  batchSize,
		FinishedAt: now,
	}
	if b.chunkBatchActive {
		summary.BatchID = b.chunkBatchCurrentID
		summary.Started = true
		summary.LoadEvents = b.chunkBatchLoadEvents
		summary.UnloadEvents = b.chunkBatchUnloadEvents
		summary.Duration = now.Sub(b.chunkBatchStartedAt)
	}

	b.lastChunkBatchSummary = summary
	b.chunkBatchActive = false
	b.chunkBatchCurrentID = 0
	b.chunkBatchStartedAt = time.Time{}
	b.chunkBatchLoadEvents = 0
	b.chunkBatchUnloadEvents = 0
	b.chunkBatchMu.Unlock()

	if !summary.Started {
		slog.Warn("Chunk batch finished received without active batch start", "batch_size", batchSize)
		return summary
	}

	slog.Debug(
		"Chunk batch finished",
		"batch_id", summary.BatchID,
		"batch_size", summary.BatchSize,
		"load_events", summary.LoadEvents,
		"unload_events", summary.UnloadEvents,
		"duration_ms", summary.Duration.Milliseconds(),
	)
	return summary
}
