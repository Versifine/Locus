package bot

import (
	"bytes"
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/Versifine/locus/internal/protocol"
	"github.com/Versifine/locus/internal/world"
)

func (b *Bot) handleLevelChunkWithLight(payload []byte) {
	if b.blockStore == nil {
		slog.Warn("Skipping chunk load because block store is not initialized")
		return
	}

	packetRdr := bytes.NewReader(payload)
	chunk, err := protocol.ParseLevelChunkWithLight(packetRdr)
	if err != nil {
		b.captureFailedChunkPayload(payload, err)
		slog.Warn("Failed to parse level chunk with light", "error", err, "payload_len", len(payload))
		return
	}

	sections, normalizeErr := normalizeSectionsForBlockStore(chunk.Sections)
	if normalizeErr != nil {
		slog.Warn(
			"Failed to normalize chunk sections for block store",
			"chunk_x", chunk.ChunkX,
			"chunk_z", chunk.ChunkZ,
			"parsed_section_count", chunk.SectionCount,
			"has_biome_data", chunk.HasBiomeData,
			"error", normalizeErr,
		)
		return
	}
	blockEntities := make([]world.BlockEntity, 0, len(chunk.BlockEntities))
	for _, be := range chunk.BlockEntities {
		blockEntities = append(blockEntities, world.BlockEntity{
			X:       be.X,
			Y:       be.Y,
			Z:       be.Z,
			TypeID:  be.TypeID,
			NBTData: be.NBTData,
		})
	}

	if err := b.blockStore.StoreChunkWithBlockEntities(chunk.ChunkX, chunk.ChunkZ, sections, blockEntities); err != nil {
		slog.Warn("Failed to store chunk", "chunk_x", chunk.ChunkX, "chunk_z", chunk.ChunkZ, "error", err)
		return
	}

	slog.Debug(
		"Stored chunk",
		"chunk_x", chunk.ChunkX,
		"chunk_z", chunk.ChunkZ,
		"section_count", chunk.SectionCount,
		"block_entity_count", len(chunk.BlockEntities),
	)
	b.noteChunkBatchChunkLoad()
	b.noteChunkLoad()
}

func (b *Bot) handlePlayLogin(payload []byte) {
	if b.worldState == nil {
		return
	}

	packetRdr := bytes.NewReader(payload)
	login, err := protocol.ParsePlayLogin(packetRdr)
	if err != nil {
		slog.Warn("Failed to parse play login", "error", err)
		return
	}

	b.setSelfEntityID(login.EntityID)
	b.worldState.UpdateDimensionContext(login.WorldState.Name, login.SimulationDistance)
	if bounds, ok := world.VanillaDimensionBounds(login.WorldState.Name); ok {
		slog.Info(
			"Updated dimension context from play login",
			"dimension", login.WorldState.Name,
			"simulation_distance", login.SimulationDistance,
			"min_y", bounds.MinY,
			"height", bounds.Height,
		)
	} else {
		slog.Warn(
			"Updated dimension context from play login with unknown dimension",
			"dimension", login.WorldState.Name,
			"simulation_distance", login.SimulationDistance,
		)
	}
}

func (b *Bot) handleRespawn(payload []byte) {
	if b.worldState == nil {
		return
	}

	packetRdr := bytes.NewReader(payload)
	respawn, err := protocol.ParseRespawn(packetRdr)
	if err != nil {
		slog.Warn("Failed to parse respawn", "error", err)
		return
	}

	current := b.worldState.GetState()
	b.worldState.UpdateDimensionContext(respawn.WorldState.Name, current.SimulationDistance)
	b.worldState.ClearEntities()
	b.resetPlayerLoaded()
	b.resetPendingDigRequests("respawn")

	if b.blockStore != nil {
		b.blockStore.Clear()
	}
	b.footLogMu.Lock()
	b.lastFootLogged = footBlockSnapshot{}
	b.footLogMu.Unlock()

	if bounds, ok := world.VanillaDimensionBounds(respawn.WorldState.Name); ok {
		slog.Info(
			"Handled respawn and cleared cached chunks",
			"dimension", respawn.WorldState.Name,
			"simulation_distance", current.SimulationDistance,
			"min_y", bounds.MinY,
			"height", bounds.Height,
		)
	} else {
		slog.Warn(
			"Handled respawn for unknown dimension and cleared cached chunks",
			"dimension", respawn.WorldState.Name,
			"simulation_distance", current.SimulationDistance,
		)
	}
}

func (b *Bot) handleUpdateViewPosition(payload []byte) {
	if b.worldState == nil {
		return
	}

	packetRdr := bytes.NewReader(payload)
	viewPos, err := protocol.ParseUpdateViewPosition(packetRdr)
	if err != nil {
		slog.Warn("Failed to parse update view position", "error", err)
		return
	}
	b.worldState.UpdateViewCenter(viewPos.ChunkX, viewPos.ChunkZ)
}

func (b *Bot) handleChunkBatchStart(payload []byte) {
	packetRdr := bytes.NewReader(payload)
	if _, err := protocol.ParseChunkBatchStart(packetRdr); err != nil {
		slog.Warn("Failed to parse chunk batch start", "error", err)
		return
	}
	b.beginChunkBatch()
}

func (b *Bot) handleChunkBatchFinished(payload []byte) {
	packetRdr := bytes.NewReader(payload)
	finished, err := protocol.ParseChunkBatchFinished(packetRdr)
	if err != nil {
		slog.Warn("Failed to parse chunk batch finished", "error", err)
		return
	}
	summary := b.finishChunkBatch(finished.BatchSize)

	if b.conn == nil || b.connState == nil {
		slog.Warn("Skipping chunk batch received ack because connection is not initialized")
		return
	}

	ack := protocol.CreateChunkBatchReceivedPacket(chunksPerTickAck)
	if err := b.writePacket(b.conn, ack, b.connState.GetThreshold()); err != nil {
		slog.Warn("Failed to send chunk batch received ack", "error", err, "batch_size", finished.BatchSize)
		return
	}
	slog.Debug(
		"Sent chunk batch received ack",
		"batch_size", finished.BatchSize,
		"chunks_per_tick", chunksPerTickAck,
		"batch_id", summary.BatchID,
		"had_start", summary.Started,
		"load_events", summary.LoadEvents,
		"unload_events", summary.UnloadEvents,
	)
}

func (b *Bot) handleUnloadChunk(payload []byte) {
	if b.blockStore == nil {
		slog.Warn("Skipping chunk unload because block store is not initialized")
		return
	}

	packetRdr := bytes.NewReader(payload)
	unload, err := protocol.ParseUnloadChunk(packetRdr)
	if err != nil {
		slog.Warn("Failed to parse unload chunk", "error", err)
		return
	}

	b.blockStore.UnloadChunk(unload.ChunkX, unload.ChunkZ)
	slog.Debug("Unloaded chunk", "chunk_x", unload.ChunkX, "chunk_z", unload.ChunkZ)
	b.noteChunkBatchChunkUnload()
	b.noteChunkUnload()
}

func (b *Bot) handleBlockChange(payload []byte) {
	if b.blockStore == nil {
		slog.Warn("Skipping block change because block store is not initialized")
		return
	}

	packetRdr := bytes.NewReader(payload)
	change, err := protocol.ParseBlockChange(packetRdr)
	if err != nil {
		slog.Warn("Failed to parse block change", "error", err)
		return
	}

	if !b.blockStore.SetBlockState(change.X, change.Y, change.Z, change.StateID) {
		return
	}

	if b.isBlockUnderFeet(change.X, change.Y, change.Z) {
		b.logBlockUnderFeetState()
	}
}

func (b *Bot) handleMultiBlockChange(payload []byte) {
	if b.blockStore == nil {
		slog.Warn("Skipping multi block change because block store is not initialized")
		return
	}

	packetRdr := bytes.NewReader(payload)
	change, err := protocol.ParseMultiBlockChange(packetRdr)
	if err != nil {
		slog.Warn("Failed to parse multi block change", "error", err)
		return
	}

	footBlockTouched := false
	for _, record := range change.Records {
		if b.blockStore.SetBlockState(record.X, record.Y, record.Z, record.StateID) {
			if b.isBlockUnderFeet(record.X, record.Y, record.Z) {
				footBlockTouched = true
			}
		}
	}
	if footBlockTouched {
		b.logBlockUnderFeetState()
	}
}

func (b *Bot) handleTileEntityData(payload []byte) {
	if b.blockStore == nil {
		slog.Warn("Skipping tile entity update because block store is not initialized")
		return
	}

	packetRdr := bytes.NewReader(payload)
	update, err := protocol.ParseTileEntityData(packetRdr)
	if err != nil {
		slog.Warn("Failed to parse tile entity data", "error", err)
		return
	}

	updated := b.blockStore.UpdateTileEntityData(
		int(update.X),
		int(update.Y),
		int(update.Z),
		update.Action,
		update.NBTData,
	)
	slog.Debug(
		"Applied tile entity data",
		"x", update.X,
		"y", update.Y,
		"z", update.Z,
		"action", update.Action,
		"has_nbt", update.NBTData != nil,
		"updated", updated,
	)
}

func (b *Bot) handleBlockAction(payload []byte) {
	if b.blockStore == nil {
		slog.Warn("Skipping block action because block store is not initialized")
		return
	}

	packetRdr := bytes.NewReader(payload)
	action, err := protocol.ParseBlockAction(packetRdr)
	if err != nil {
		slog.Warn("Failed to parse block action", "error", err)
		return
	}

	recorded := b.blockStore.RecordBlockAction(
		int(action.X),
		int(action.Y),
		int(action.Z),
		action.Byte1,
		action.Byte2,
		action.BlockID,
	)
	slog.Debug(
		"Recorded block action",
		"x", action.X,
		"y", action.Y,
		"z", action.Z,
		"byte1", action.Byte1,
		"byte2", action.Byte2,
		"block_id", action.BlockID,
		"recorded", recorded,
	)
}

func (b *Bot) logBlockUnderFeetState() {
	if b.blockStore == nil || b.worldState == nil {
		return
	}

	pos := b.worldState.GetState().Position
	blockX := int(math.Floor(pos.X))
	blockY := int(math.Floor(pos.Y)) - 1
	blockZ := int(math.Floor(pos.Z))

	stateID, ok := b.blockStore.GetBlockState(blockX, blockY, blockZ)
	if !ok {
		return
	}
	blockName, ok := b.blockStore.GetBlockNameByStateID(stateID)
	if !ok {
		blockName = "Unknown"
	}

	current := footBlockSnapshot{
		X:       blockX,
		Y:       blockY,
		Z:       blockZ,
		StateID: stateID,
		Valid:   true,
	}

	b.footLogMu.Lock()
	if b.lastFootLogged.Valid &&
		b.lastFootLogged.X == current.X &&
		b.lastFootLogged.Y == current.Y &&
		b.lastFootLogged.Z == current.Z &&
		b.lastFootLogged.StateID == current.StateID {
		b.footLogMu.Unlock()
		return
	}
	b.lastFootLogged = current
	b.footLogMu.Unlock()

	slog.Info("Block under feet",
		"x", blockX,
		"y", blockY,
		"z", blockZ,
		"state_id", stateID,
		"block_name", blockName,
	)
}

func (b *Bot) isBlockUnderFeet(x, y, z int) bool {
	if b.worldState == nil {
		return false
	}
	pos := b.worldState.GetState().Position
	return int(math.Floor(pos.X)) == x &&
		int(math.Floor(pos.Y))-1 == y &&
		int(math.Floor(pos.Z)) == z
}

func (b *Bot) logBlockUnderFeetLoop(ctx context.Context) {
	ticker := time.NewTicker(footBlockLogInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.logBlockUnderFeetState()
		}
	}
}
