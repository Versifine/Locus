package bot

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/protocol"
	"github.com/Versifine/locus/internal/world"
)

const (
	defaultChunkCaptureDir = "temp/chunk_payloads"
	defaultChunkCaptureMax = 8
	footBlockLogInterval   = 2 * time.Second
	chunksPerTickAck       = float32(20.0)
)

type Bot struct {
	serverAddr string
	username   string
	uuid       protocol.UUID
	conn       net.Conn
	connState  *protocol.ConnState
	eventBus   *event.Bus
	worldState *world.WorldState
	blockStore *world.BlockStore
	injectCh   chan string
	mu         sync.RWMutex

	chunkCaptureMu    sync.Mutex
	chunkCaptureDir   string
	chunkCaptureMax   int
	chunkCaptureCount int

	footLogMu      sync.Mutex
	lastFootLogged footBlockSnapshot

	chunkStatsMu        sync.Mutex
	lastChunkStatsLogAt time.Time
	chunkLoadEvents     int
	chunkUnloadEvents   int

	unhandledMu           sync.Mutex
	unhandledPacketCounts map[int32]int

	chunkBatchMu           sync.Mutex
	chunkBatchSeq          int64
	chunkBatchActive       bool
	chunkBatchCurrentID    int64
	chunkBatchStartedAt    time.Time
	chunkBatchLoadEvents   int
	chunkBatchUnloadEvents int
	lastChunkBatchSummary  chunkBatchSummary

	playerLoadedMu   sync.Mutex
	sentPlayerLoaded bool
}

type footBlockSnapshot struct {
	X       int
	Y       int
	Z       int
	StateID int32
	Valid   bool
}

type chunkBatchSummary struct {
	BatchID      int64
	Started      bool
	BatchSize    int32
	LoadEvents   int
	UnloadEvents int
	Duration     time.Duration
	FinishedAt   time.Time
}

func NewBot(serverAddr, username string) *Bot {
	blockStore, err := world.NewBlockStore()
	if err != nil {
		slog.Error("Failed to initialize block store", "error", err)
	}
	return &Bot{
		serverAddr: serverAddr,
		username:   username,
		uuid:       protocol.GenerateOfflineUUID(username),
		eventBus:   event.NewBus(),
		injectCh:   make(chan string, 100),
		worldState: &world.WorldState{},
		blockStore: blockStore,

		chunkCaptureDir: defaultChunkCaptureDir,
		chunkCaptureMax: defaultChunkCaptureMax,
	}
}

func (b *Bot) Start(ctx context.Context) error {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", b.serverAddr)
	if err != nil {
		return err
	}
	slog.Info("Connected to server", "address", b.serverAddr)
	b.conn = conn
	defer conn.Close()
	b.connState = protocol.NewConnState()
	//handshake and login
	if err := b.login(); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	//configuration
	if err := b.handleConfiguration(); err != nil {
		return fmt.Errorf("configuration failed: %w", err)
	}
	//start play state handler
	go b.handleInjection(ctx)
	go b.logBlockUnderFeetLoop(ctx)

	return b.handlePlayState(ctx)
}

func (b *Bot) login() error {
	host, portStr, err := net.SplitHostPort(b.serverAddr)
	if err != nil {
		return fmt.Errorf("invalid server address: %w", err)
	}
	port, _ := strconv.ParseUint(portStr, 10, 16)
	// 发送握手包和登录开始包
	slog.Info("Starting Handshake", "state", "Handshake")
	handshakePacket := protocol.CreateHandshakePacket(protocol.CurrentProtocolVersion, host, uint16(port), protocol.NextStateLogin)
	if err := protocol.WritePacket(b.conn, handshakePacket, b.connState.GetThreshold()); err != nil {
		return err
	}
	b.connState.Set(protocol.Login)
	slog.Info("Starting Login", "state", "Login")
	loginStartPacket := protocol.CreateLoginStartPacket(b.username, b.uuid)
	if err := protocol.WritePacket(b.conn, loginStartPacket, b.connState.GetThreshold()); err != nil {
		return err
	}
	for {
		packet, err := protocol.ReadPacket(b.conn, b.connState.GetThreshold())
		if err != nil {
			return err
		}
		switch packet.ID {
		case protocol.S2CSetCompression:
			// 设置压缩
			slog.Info("Setting compression", "threshold", packet.Payload)
			packetRdr := bytes.NewReader(packet.Payload)
			threshold, err := protocol.ReadVarint(packetRdr)
			if err != nil {
				return err
			}
			b.connState.SetThreshold(int(threshold))

		case protocol.S2CLoginSuccess:
			// 登录成功
			packetRdr := bytes.NewReader(packet.Payload)
			loginSuccess, err := protocol.ParseLoginSuccess(packetRdr)
			if err != nil {
				return err
			}
			b.connState.SetUsername(loginSuccess.Username)
			b.connState.SetUUID(loginSuccess.UUID)
			b.uuid = loginSuccess.UUID
			b.username = loginSuccess.Username

			loginAckPacket := protocol.CreateLoginAcknowledgedPacket()
			if err := protocol.WritePacket(b.conn, loginAckPacket, b.connState.GetThreshold()); err != nil {
				return err
			}
			b.connState.Set(protocol.Configuration)
			slog.Info("Login successful", "username", loginSuccess.Username, "uuid", loginSuccess.UUID.String())
			return nil
		}
	}
}

func (b *Bot) handleConfiguration() error {
	clientInfoPack := protocol.CreateClientInformationPacket("zh_cn", 10, 0, true, 127, 1, false, true, 0, protocol.C2SConfigClientInformation)
	if err := b.writePacket(b.conn, clientInfoPack, b.connState.GetThreshold()); err != nil {
		return err
	}
	slog.Info("Starting Configuration", "state", "Configuration")
	for {
		packet, err := protocol.ReadPacket(b.conn, b.connState.GetThreshold())
		if err != nil {
			return err
		}
		switch packet.ID {
		case protocol.S2CConfigKeepAlive:
			// 响应保持连接包
			packetRdr := bytes.NewReader(packet.Payload)
			keepAlivePacket, err := protocol.ParseKeepAlive(packetRdr)
			if err != nil {
				return err
			}
			keepAliveResponsePacket := protocol.CreateKeepAlivePacket(keepAlivePacket.KeepAliveID, protocol.C2SConfigKeepAlive)
			if err := b.writePacket(b.conn, keepAliveResponsePacket, b.connState.GetThreshold()); err != nil {
				return err
			}

		case protocol.S2CSelectKnown:

			// 选择已知的配置选项
			knownPacks := []protocol.KnownPack{
				{NameSpace: "minecraft",
					Id:      "locus",
					Version: "1.21.11"},
			}

			selectKnownPacket := protocol.CreateSelectKnownPacket(knownPacks, protocol.C2SSelectKnown) // 示例选择第一个已知选项
			if err := b.writePacket(b.conn, selectKnownPacket, b.connState.GetThreshold()); err != nil {
				return err
			}
		case protocol.S2CFinishConfiguration:
			// 完成配置，进入游戏状态
			ack := protocol.CreateFinishConfigurationPacket(protocol.C2SFinishConfiguration)
			if err := b.writePacket(b.conn, ack, b.connState.GetThreshold()); err != nil {
				return err
			}

			b.connState.Set(protocol.Play)
			return nil
		}
	}
}

func (b *Bot) handlePlayState(ctx context.Context) error {
	slog.Info("Starting Play", "state", "Play")
	for {
		packet, err := protocol.ReadPacket(b.conn, b.connState.GetThreshold())
		if err != nil {
			return err
		}
		switch packet.ID {
		case protocol.S2CPlayKeepAlive:
			// 响应保持连接包
			packetRdr := bytes.NewReader(packet.Payload)
			keepAlivePacket, err := protocol.ParseKeepAlive(packetRdr)
			if err != nil {
				return err
			}
			keepAliveResponsePacket := protocol.CreateKeepAlivePacket(keepAlivePacket.KeepAliveID, protocol.C2SPlayKeepAlive)
			if err := b.writePacket(b.conn, keepAliveResponsePacket, b.connState.GetThreshold()); err != nil {
				return err
			}
		case protocol.S2CPlayerChatMessage:
			// 处理聊天消息
			packetRdr := bytes.NewReader(packet.Payload)
			playerChat, err := protocol.ParsePlayerChat(packetRdr)
			if err != nil {
				slog.Warn("Failed to parse player chat", "error", err)
				continue
			}
			if playerChat.SenderUUID == b.uuid {
				// 忽略自己的消息
				continue
			}
			b.eventBus.Publish(event.EventChat, event.NewChatEvent(ctx, protocol.FormatTextComponent(playerChat.NetworkName), playerChat.SenderUUID, playerChat.PlainMessage, event.SourcePlayer))

		case protocol.S2CSystemChatMessage:
			// 处理系统消息
		case protocol.S2CLogin:
			b.handlePlayLogin(packet.Payload)
		case protocol.S2CRespawn:
			b.handleRespawn(packet.Payload)
		case protocol.S2CUpdateViewPosition:
			b.handleUpdateViewPosition(packet.Payload)
		case protocol.S2CChunkBatchStart:
			b.handleChunkBatchStart(packet.Payload)
		case protocol.S2CChunkBatchFinished:
			b.handleChunkBatchFinished(packet.Payload)
		case protocol.S2CPlayerPosition:
			// 处理玩家位置更新
			packetRdr := bytes.NewReader(packet.Payload)
			playerPos, err := protocol.ParsePlayerPosition(packetRdr)
			if err != nil {
				return err
			}
			teleCfmPacket := protocol.CreateTeleportConfirmPacket(playerPos.TeleportID)
			b.worldState.UpdatePosition(world.Position{
				X:     playerPos.X,
				Y:     playerPos.Y,
				Z:     playerPos.Z,
				Yaw:   playerPos.Yaw,
				Pitch: playerPos.Pitch,
			})
			b.logBlockUnderFeetState()
			if err := b.writePacket(b.conn, teleCfmPacket, b.connState.GetThreshold()); err != nil {
				return err
			}
			// Vanilla clients send a movement packet after teleport confirm.
			// Some servers rely on this to continue chunk streaming.
			posAck := protocol.CreatePlayerPositionAndRotationPacket(
				playerPos.X,
				playerPos.Y,
				playerPos.Z,
				playerPos.Yaw,
				playerPos.Pitch,
				false,
				false,
			)
			if err := b.writePacket(b.conn, posAck, b.connState.GetThreshold()); err != nil {
				return err
			}
			if err := b.maybeSendPlayerLoaded(); err != nil {
				return err
			}
		case protocol.S2CLevelChunkWithLight:
			b.handleLevelChunkWithLight(packet.Payload)
		case protocol.S2CUnloadChunk:
			b.handleUnloadChunk(packet.Payload)
		case protocol.S2CBlockChange:
			b.handleBlockChange(packet.Payload)
		case protocol.S2CMultiBlockChange:
			b.handleMultiBlockChange(packet.Payload)
		case protocol.S2CTileEntityData:
			b.handleTileEntityData(packet.Payload)
		case protocol.S2CBlockAction:
			b.handleBlockAction(packet.Payload)
		case protocol.S2CUpdateHealth:
			packetRdr := bytes.NewReader(packet.Payload)
			updateHealth, err := protocol.ParseUpdateHealth(packetRdr)
			if err != nil {
				return err
			}
			b.worldState.UpdateHealth(updateHealth.Health, updateHealth.Food)
			if updateHealth.Health <= 0 {
				slog.Info("Bot died, sending respawn")
				respawnPacket := protocol.CreateClientCommandPacket(0)
				if err := b.writePacket(b.conn, respawnPacket, b.connState.GetThreshold()); err != nil {
					return err
				}
			}
		case protocol.S2CUpdateTime:
			// 处理时间更新
			packetRdr := bytes.NewReader(packet.Payload)
			updateTime, err := protocol.ParseUpdateTime(packetRdr)
			if err != nil {
				return err
			}
			b.worldState.UpdateGameTime(world.GameTime{
				WorldTime: updateTime.WorldTime,
				Age:       updateTime.Age,
			})
		case protocol.S2CPlayerInfo:
			// 处理玩家信息更新
			packetRdr := bytes.NewReader(packet.Payload)
			playerInfoUpdate, err := protocol.ParsePlayerInfo(packetRdr)
			if err != nil {
				slog.Error("Failed to parse player info", "error", err)
				continue
			}
			addPlayerList := make([]world.Player, 0)
			if playerInfoUpdate.Actions&0x01 != 0 {
				for _, p := range playerInfoUpdate.Players {
					addPlayerList = append(addPlayerList, world.Player{
						Name: p.Name,
						UUID: p.UUID.String(),
					})
				}
			}
			if len(addPlayerList) > 0 {
				b.worldState.AddPlayer(addPlayerList)
			}
		case protocol.S2CPlayerRemove:
			// 处理玩家移除
			packetRdr := bytes.NewReader(packet.Payload)
			playerRemove, err := protocol.ParsePlayerRemove(packetRdr)
			if err != nil {
				return err
			}
			for _, uuid := range playerRemove.Players {
				b.worldState.RemovePlayer(uuid.String())
			}
		case protocol.S2CSpawnEntity:
			packetRdr := bytes.NewReader(packet.Payload)
			spawn, err := protocol.ParseSpawnEntity(packetRdr)
			if err != nil {
				slog.Warn("Failed to parse spawn entity", "error", err)
				continue
			}
			b.worldState.AddEntity(world.Entity{
				EntityID: spawn.EntityID,
				UUID:     spawn.ObjectUUID.String(),
				Type:     spawn.Type,
				X:        spawn.X,
				Y:        spawn.Y,
				Z:        spawn.Z,
			})
		case protocol.S2CEntityMetadata:
			packetRdr := bytes.NewReader(packet.Payload)
			entityID, itemID, found, err := protocol.ParseEntityMetadataItemSlot(packetRdr)
			if err != nil {
				slog.Warn("Failed to parse entity metadata", "error", err)
				continue
			}
			if found {
				itemName := world.ItemName(itemID)
				b.worldState.UpdateEntityItemName(entityID, itemName)
			}
		case protocol.S2CEntityDestroy:
			packetRdr := bytes.NewReader(packet.Payload)
			destroy, err := protocol.ParseEntityDestroy(packetRdr)
			if err != nil {
				slog.Warn("Failed to parse entity destroy", "error", err)
				continue
			}
			b.worldState.RemoveEntities(destroy.EntityIDs)
		case protocol.S2CRelEntityMove:
			packetRdr := bytes.NewReader(packet.Payload)
			move, err := protocol.ParseRelEntityMove(packetRdr)
			if err != nil {
				slog.Warn("Failed to parse rel entity move", "error", err)
				continue
			}
			b.worldState.UpdateEntityPositionRelative(move.EntityID, move.DeltaX(), move.DeltaY(), move.DeltaZ())
		case protocol.S2CEntityMoveLook:
			packetRdr := bytes.NewReader(packet.Payload)
			move, err := protocol.ParseEntityMoveLook(packetRdr)
			if err != nil {
				slog.Warn("Failed to parse entity move look", "error", err)
				continue
			}
			b.worldState.UpdateEntityPositionRelative(move.EntityID, move.DeltaX(), move.DeltaY(), move.DeltaZ())
		case protocol.S2CEntityTeleport:
			packetRdr := bytes.NewReader(packet.Payload)
			tp, err := protocol.ParseEntityTeleport(packetRdr)
			if err != nil {
				slog.Warn("Failed to parse entity teleport", "error", err)
				continue
			}
			b.worldState.UpdateEntityPosition(tp.EntityID, tp.X, tp.Y, tp.Z)
		case protocol.S2CSyncEntityPosition:
			packetRdr := bytes.NewReader(packet.Payload)
			syncPos, err := protocol.ParseSyncEntityPosition(packetRdr)
			if err != nil {
				slog.Warn("Failed to parse sync entity position", "error", err)
				continue
			}
			b.worldState.UpdateEntityPosition(syncPos.EntityID, syncPos.X, syncPos.Y, syncPos.Z)
		default:
			b.logUnhandledPlayPacket(packet.ID)
		}

	}
}

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
	b.resetPlayerLoaded()

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
