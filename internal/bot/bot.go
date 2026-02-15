package bot

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
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
	connectionState
	runtimeState
	chunkCaptureState
	footLogState
	chunkStatsState
	unhandledPacketState
	chunkBatchState
	playerLoadedState
	digSyncState
}

type connectionState struct {
	serverAddr string
	username   string
	uuid       protocol.UUID
	conn       net.Conn
	connState  *protocol.ConnState
	mu         sync.RWMutex
}

type runtimeState struct {
	eventBus   *event.Bus
	worldState *world.WorldState
	blockStore *world.BlockStore
	injectCh   chan string
}

type chunkCaptureState struct {
	chunkCaptureMu    sync.Mutex
	chunkCaptureDir   string
	chunkCaptureMax   int
	chunkCaptureCount int
}

type footLogState struct {
	footLogMu      sync.Mutex
	lastFootLogged footBlockSnapshot
}

type chunkStatsState struct {
	chunkStatsMu        sync.Mutex
	lastChunkStatsLogAt time.Time
	chunkLoadEvents     int
	chunkUnloadEvents   int
}

type unhandledPacketState struct {
	unhandledMu           sync.Mutex
	unhandledPacketCounts map[int32]int
}

type chunkBatchState struct {
	chunkBatchMu           sync.Mutex
	chunkBatchSeq          int64
	chunkBatchActive       bool
	chunkBatchCurrentID    int64
	chunkBatchStartedAt    time.Time
	chunkBatchLoadEvents   int
	chunkBatchUnloadEvents int
	lastChunkBatchSummary  chunkBatchSummary
}

type playerLoadedState struct {
	playerLoadedMu   sync.Mutex
	sentPlayerLoaded bool
}

type digSyncState struct {
	digMu              sync.Mutex
	nextDigSequence    int32
	pendingDigRequests map[int32]pendingDigRequest
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

type pendingDigRequest struct {
	Status   int32
	Location protocol.BlockPos
	Face     int8
	SentAt   time.Time
}

func NewBot(serverAddr, username string) *Bot {
	blockStore, err := world.NewBlockStore()
	if err != nil {
		slog.Error("Failed to initialize block store", "error", err)
	}
	return &Bot{
		connectionState: connectionState{
			serverAddr: serverAddr,
			username:   username,
			uuid:       protocol.GenerateOfflineUUID(username),
		},
		runtimeState: runtimeState{
			eventBus:   event.NewBus(),
			worldState: &world.WorldState{},
			blockStore: blockStore,
			injectCh:   make(chan string, 100),
		},
		chunkCaptureState: chunkCaptureState{
			chunkCaptureDir: defaultChunkCaptureDir,
			chunkCaptureMax: defaultChunkCaptureMax,
		},
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
		case protocol.S2CAcknowledgePlayerDigging:
			b.handleAcknowledgePlayerDigging(packet.Payload)
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
