package bot

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/protocol"
	"github.com/Versifine/locus/internal/world"
)

func (b *Bot) handleInjection(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-b.injectCh:
			slog.Info("Injecting message", "message", msg)
			chatPacket := protocol.CreateChatMessagePacket(msg)
			if err := b.writePacket(b.conn, chatPacket, b.connState.GetThreshold()); err != nil {
				slog.Error("Failed to inject message", "error", err)
			}
		}
	}
}

func (b *Bot) writePacket(w io.Writer, packet *protocol.Packet, threshold int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return protocol.WritePacket(w, packet, threshold)
}

func (b *Bot) Bus() *event.Bus {
	return b.eventBus
}

func (b *Bot) SendMsgToServer(msg string) error {
	b.injectCh <- msg
	return nil
}

func (b *Bot) GetState() world.Snapshot {
	return b.worldState.GetState()
}

func (b *Bot) GetBlockState(x, y, z int) (int32, bool) {
	if b.blockStore == nil {
		return 0, false
	}
	return b.blockStore.GetBlockState(x, y, z)
}

func (b *Bot) logUnhandledPlayPacket(packetID int32) {
	b.unhandledMu.Lock()
	defer b.unhandledMu.Unlock()

	if b.unhandledPacketCounts == nil {
		b.unhandledPacketCounts = make(map[int32]int)
	}
	b.unhandledPacketCounts[packetID]++
	count := b.unhandledPacketCounts[packetID]

	// Log first sighting of packet ID and then every 100 repeats.
	if count == 1 || count%100 == 0 {
		slog.Debug("Unhandled packet in Play state", "packet_id", fmt.Sprintf("0x%02x", packetID), "count", count)
	}
}

func (b *Bot) maybeSendPlayerLoaded() error {
	b.playerLoadedMu.Lock()
	alreadySent := b.sentPlayerLoaded
	if !alreadySent {
		b.sentPlayerLoaded = true
	}
	b.playerLoadedMu.Unlock()

	if alreadySent {
		return nil
	}

	packet := protocol.CreatePlayerLoadedPacket()
	if err := b.writePacket(b.conn, packet, b.connState.GetThreshold()); err != nil {
		b.playerLoadedMu.Lock()
		b.sentPlayerLoaded = false
		b.playerLoadedMu.Unlock()
		return err
	}

	slog.Debug("Sent player loaded packet")
	return nil
}

func (b *Bot) resetPlayerLoaded() {
	b.playerLoadedMu.Lock()
	defer b.playerLoadedMu.Unlock()
	b.sentPlayerLoaded = false
}
