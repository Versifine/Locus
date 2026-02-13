package bot

import (
	"bytes"
	"fmt"
	"log/slog"
	"time"

	"github.com/Versifine/locus/internal/protocol"
)

func (b *Bot) handleAcknowledgePlayerDigging(payload []byte) {
	packetRdr := bytes.NewReader(payload)
	ack, err := protocol.ParseAcknowledgePlayerDigging(packetRdr)
	if err != nil {
		slog.Warn("Failed to parse acknowledge player digging", "error", err)
		return
	}

	b.digMu.Lock()
	req, ok := b.pendingDigRequests[ack.SequenceID]
	if ok {
		delete(b.pendingDigRequests, ack.SequenceID)
	}
	b.digMu.Unlock()

	if !ok {
		slog.Warn("Received unknown or stale digging ack", "sequence_id", ack.SequenceID)
		return
	}

	slog.Debug(
		"Digging ack reconciled",
		"sequence_id", ack.SequenceID,
		"status", req.Status,
		"x", req.Location.X,
		"y", req.Location.Y,
		"z", req.Location.Z,
		"face", req.Face,
		"latency_ms", time.Since(req.SentAt).Milliseconds(),
	)
}

func (b *Bot) SendBlockDig(status int32, location protocol.BlockPos, face int8) (int32, error) {
	if b.conn == nil || b.connState == nil {
		return 0, fmt.Errorf("connection is not initialized")
	}

	b.digMu.Lock()
	sequence := b.nextDigSequence
	b.nextDigSequence++
	if b.pendingDigRequests == nil {
		b.pendingDigRequests = make(map[int32]pendingDigRequest)
	}
	b.pendingDigRequests[sequence] = pendingDigRequest{
		Status:   status,
		Location: location,
		Face:     face,
		SentAt:   time.Now(),
	}
	b.digMu.Unlock()

	packet := protocol.CreateBlockDigPacket(status, location, face, sequence)
	if err := b.writePacket(b.conn, packet, b.connState.GetThreshold()); err != nil {
		b.digMu.Lock()
		delete(b.pendingDigRequests, sequence)
		b.digMu.Unlock()
		return 0, err
	}

	slog.Debug(
		"Sent block dig",
		"sequence_id", sequence,
		"status", status,
		"x", location.X,
		"y", location.Y,
		"z", location.Z,
		"face", face,
	)
	return sequence, nil
}

func (b *Bot) resetPendingDigRequests(reason string) {
	b.digMu.Lock()
	pendingCount := len(b.pendingDigRequests)
	if pendingCount > 0 {
		b.pendingDigRequests = make(map[int32]pendingDigRequest)
	}
	b.digMu.Unlock()

	if pendingCount > 0 {
		slog.Warn("Cleared pending dig requests", "reason", reason, "count", pendingCount)
	}
}
