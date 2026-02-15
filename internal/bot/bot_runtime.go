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

func (b *Bot) SendPacket(packet *protocol.Packet) error {
	if packet == nil {
		return fmt.Errorf("packet is nil")
	}
	if b.conn == nil || b.connState == nil {
		return fmt.Errorf("connection is not initialized")
	}
	return b.writePacket(b.conn, packet, b.connState.GetThreshold())
}

func (b *Bot) SetLocalPositionSink(sink interface{ SetLocalPosition(pos world.Position) }) {
	b.localPosSinkMu.Lock()
	defer b.localPosSinkMu.Unlock()
	b.localPosSink = sink
}

func (b *Bot) setSelfEntityID(entityID int32) {
	b.selfEntityMu.Lock()
	b.selfEntityID = entityID
	b.hasSelfEntity = true
	b.selfEntityMu.Unlock()
}

func (b *Bot) SelfEntityID() (int32, bool) {
	b.selfEntityMu.RLock()
	defer b.selfEntityMu.RUnlock()
	return b.selfEntityID, b.hasSelfEntity
}

func (b *Bot) WaitForInitialPosition(ctx context.Context) error {
	if b.initialPosCh == nil {
		return fmt.Errorf("initial position channel is not initialized")
	}
	select {
	case <-b.initialPosCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *Bot) markInitialPositionReady() {
	if b.initialPosCh == nil {
		return
	}
	b.initialPosOnce.Do(func() {
		close(b.initialPosCh)
	})
}

func (b *Bot) syncLocalPosition(pos world.Position) {
	b.localPosSinkMu.RLock()
	sink := b.localPosSink
	b.localPosSinkMu.RUnlock()
	if sink != nil {
		sink.SetLocalPosition(pos)
	}
}

func (b *Bot) GetState() world.Snapshot {
	return b.worldState.GetState()
}

func (b *Bot) UpdatePosition(pos world.Position) {
	if b.worldState == nil {
		return
	}
	b.worldState.UpdatePosition(pos)
}

func (b *Bot) IsSolid(x, y, z int) bool {
	if b.blockStore == nil {
		return false
	}
	return b.blockStore.IsSolid(x, y, z)
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
