package bot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/protocol"
	"github.com/Versifine/locus/internal/world"
)

type Bot struct {
	serverAddr string
	username   string
	uuid       protocol.UUID
	conn       net.Conn
	connState  *protocol.ConnState
	eventBus   *event.Bus
	worldState *world.WorldState
	injectCh   chan string
	mu         sync.RWMutex
}

func NewBot(serverAddr, username string) *Bot {
	return &Bot{
		serverAddr: serverAddr,
		username:   username,
		uuid:       protocol.GenerateOfflineUUID(username),
		eventBus:   event.NewBus(),
		injectCh:   make(chan string, 100),
		worldState: &world.WorldState{},
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
		slog.Debug("Received packet in Login state", "packet_id", packet.ID)
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
		slog.Debug("Received packet in Configuration state", "packet_id", packet.ID)
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
		slog.Debug("Received packet in Play state", "packet_id", packet.ID)
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
			if err := b.writePacket(b.conn, teleCfmPacket, b.connState.GetThreshold()); err != nil {
				return err
			}
		case protocol.S2CUpdateHealth:
			// 处理健康和饥饿更新
			packetRdr := bytes.NewReader(packet.Payload)
			updateHealth, err := protocol.ParseUpdateHealth(packetRdr)
			if err != nil {
				return err
			}
			b.worldState.UpdateHealth(updateHealth.Health, updateHealth.Food)
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
		default:
			slog.Debug("Unhandled packet in Play state", "packet_id", packet.ID)
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
