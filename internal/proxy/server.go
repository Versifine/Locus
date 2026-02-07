package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/protocol"
)

type Server struct {
	listenerAddr string
	backendAddr  string
	eventBus     *event.Bus
	injectCh     chan string
	mu           sync.Mutex
}

func NewServer(listenerAddr, backendAddr string) *Server {
	return &Server{listenerAddr: listenerAddr, backendAddr: backendAddr, eventBus: event.NewBus(), injectCh: make(chan string, 100)}
}

func (s *Server) Bus() *event.Bus {
	return s.eventBus
}

func (s *Server) Start(ctx context.Context) error {
	slog.Info("Proxy server starting", "listen", s.listenerAddr, "backend", s.backendAddr)
	netListener, err := net.Listen("tcp", s.listenerAddr)
	if err != nil {
		return err
	}
	defer netListener.Close()

	go func() {
		<-ctx.Done()
		slog.Info("Proxy server shutting down")
		_ = netListener.Close()
	}()
	for {
		conn, err := netListener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				slog.Info("Proxy server stopped")
				return nil
			}
			slog.Error("Failed to accept connection", "error", err)
			return err
		}
		go s.handleConnection(ctx, conn)
	}
}

func (s *Server) SendMsgToServer(msg string) {
	s.injectCh <- msg
}

func (s *Server) handleInjects(backendConn net.Conn, ctx context.Context, connState *protocol.ConnState) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-s.injectCh:
			slog.Info("Injecting message to server", "message", msg)
			packet := protocol.CreateSayChatCommand(msg)

			s.mu.Lock()
			err := protocol.WritePacket(backendConn, packet, connState.GetThreshold())
			s.mu.Unlock()
			if err != nil {
				slog.Error("Failed to inject message to server", "error", err)
			}
		}
	}
}

func (s *Server) handleConnection(ctx context.Context, clientConn net.Conn) {
	// Disable Nagle's algorithm for lower latency
	defer clientConn.Close()
	if tcpConn, ok := clientConn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}
	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()

	backendConn, err := net.Dial("tcp", s.backendAddr)

	connState := protocol.NewConnState()
	connState.Set(protocol.Handshaking)

	if err != nil {
		slog.Error("Failed to connect backend", "backend", s.backendAddr, "error", err)
		clientConn.Close()
		return
	}
	go func() {
		s.handleInjects(backendConn, connCtx, connState)
	}()
	// Disable Nagle's algorithm for backend connection too
	if tcpConn, ok := backendConn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}

	defer backendConn.Close()
	slog.Info("New connection", "client", clientConn.RemoteAddr(), "backend", s.backendAddr)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := s.relayPackets(connCtx, clientConn, backendConn, "C->S", connState)
		if err != nil {
			slog.Error("Relay error", "dir", "C->S", "error", err)
		}
	}()
	go func() {
		defer wg.Done()
		err := s.relayPackets(connCtx, backendConn, clientConn, "S->C", connState)
		if err != nil {
			slog.Error("Relay error", "dir", "S->C", "error", err)
		}
	}()
	wg.Wait()
	slog.Info("Connection closed", "client", clientConn.RemoteAddr())
}

func (s *Server) relayPackets(ctx context.Context, src, dst net.Conn, tag string, connState *protocol.ConnState) error {
	for {
		// Read with current threshold
		currentThreshold := connState.GetThreshold()
		packet, err := protocol.ReadPacket(src, currentThreshold)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		slog.Debug("Packet received", "dir", tag, "id", fmt.Sprintf("0x%02x", packet.ID))

		var newThreshold int = -2 // -2 means no change detected

		switch connState.Get() {
		case protocol.Handshaking:
			if err := s.handleHandshaking(packet, tag, connState); err != nil {
				return err
			}
		//case protocol.Status:
		// Handle status state if needed
		case protocol.Login:
			if err := s.handleLogin(packet, tag, connState, &newThreshold); err != nil {
				return err
			}
		case protocol.Configuration:
			if err := s.handleConfiguration(packet, tag, connState); err != nil {
				return err
			}
		case protocol.Play:
			if err := s.handlePlay(ctx, packet, tag, connState); err != nil {
				return err
			}
		default:
			slog.Warn("Unknown connection state", "state", connState.Get())
		}

		// Write with current (OLD) threshold
		switch tag {
		case "C->S":
			func() {
				s.mu.Lock()
				defer s.mu.Unlock()
				if err := protocol.WritePacket(dst, packet, currentThreshold); err != nil {
					slog.Error("Failed to write packet to server", "error", err)
				}
			}()
		case "S->C":
			if err := protocol.WritePacket(dst, packet, currentThreshold); err != nil {
				return err
			}
		default:
			slog.Error("Unknown Tag")
		}

		if newThreshold != -2 {
			connState.SetThreshold(newThreshold)
		}
	}
}

func (s *Server) handleHandshaking(packet *protocol.Packet, tag string, connState *protocol.ConnState) error {
	if tag == "C->S" && packet.ID == protocol.C2SHandshake {
		packetRdr := bytes.NewReader(packet.Payload)
		handshake, err := protocol.ParseHandshake(packetRdr)
		if err != nil {
			return err
		}
		slog.Info("Handshake", "proto", handshake.ProtocolVersion, "addr", handshake.ServerAddress, "port", handshake.ServerPort, "next", handshake.NextState)
		if handshake.NextState == 1 {
			connState.Set(protocol.Status)
		}
		if handshake.NextState == 2 {
			connState.Set(protocol.Login)
		}
	}
	return nil
}

func (s *Server) handleLogin(packet *protocol.Packet, tag string, connState *protocol.ConnState, newThreshold *int) error {
	if tag == "C->S" && packet.ID == protocol.C2SLoginStart {
		packetRdr := bytes.NewReader(packet.Payload)
		loginStart, err := protocol.ParseLoginStart(packetRdr)
		if err != nil {
			return err
		}
		connState.SetUsername(loginStart.Username)
		connState.SetUUID(loginStart.UUID)
		slog.Info("Login start", "username", loginStart.Username, "uuid", loginStart.UUID.String())
	}
	if tag == "S->C" && packet.ID == protocol.S2CSetCompression {
		packetRdr := bytes.NewReader(packet.Payload)
		threshold, err := protocol.ReadVarint(packetRdr)
		if err == nil {
			*newThreshold = int(threshold)
			slog.Info("Compression enabled", "threshold", *newThreshold)

		} else {
			slog.Error("Failed to parse compression threshold", "error", err)
		}
	}
	if tag == "S->C" && packet.ID == protocol.S2CLoginSuccess {
		packetRdr := bytes.NewReader(packet.Payload)
		loginSuccess, err := protocol.ParseLoginSuccess(packetRdr)
		if err != nil {
			return err
		}
		connState.SetUUID(loginSuccess.UUID)
		connState.SetUsername(loginSuccess.Username)
		slog.Info("Login success, entering Configuration state", "username", loginSuccess.Username, "uuid", loginSuccess.UUID.String())
		connState.Set(protocol.Configuration)
	}
	return nil
}
func (s *Server) handleConfiguration(packet *protocol.Packet, tag string, connState *protocol.ConnState) error {
	// S->C 0x03 = Finish Configuration
	if tag == "S->C" && packet.ID == protocol.S2CFinishConfiguration {
		slog.Info("Finish Configuration, entering Play state")
		connState.Set(protocol.Play)
	}
	return nil
}

func (s *Server) handlePlay(ctx context.Context, packet *protocol.Packet, tag string, connState *protocol.ConnState) error {
	if tag == "S->C" && packet.ID == protocol.S2CPlayerChatMessage {
		packetRdr := bytes.NewReader(packet.Payload)
		playerChat, err := protocol.ParsePlayerChat(packetRdr)
		if err != nil {
			slog.Warn("Failed to parse Player Chat", "error", err)
			return nil
		}
		s.eventBus.Publish(event.EventChat, event.NewChatEvent(ctx, protocol.FormatTextComponent(playerChat.NetworkName), playerChat.SenderUUID, playerChat.PlainMessage, event.SourcePlayer))
	}
	if tag == "S->C" && packet.ID == protocol.S2CSystemChatMessage {
		packetRdr := bytes.NewReader(packet.Payload)
		systemChat, err := protocol.ParseSystemChat(packetRdr)
		if err != nil {
			slog.Warn("Failed to parse System Chat", "error", err)
			return nil
		}
		s.eventBus.Publish(event.EventChat, event.NewChatEvent(ctx, "SYSTEM", protocol.UUID{}, protocol.FormatTextComponent(&systemChat.Content), event.SourceSystem))
	}
	// Chat Message (C->S 0x08, Chat Command (C->S 0x06), Chat Command Signed (C->S 0x07)
	if tag == "C->S" && packet.ID == protocol.C2SChatMessage {
		packetRdr := bytes.NewReader(packet.Payload)
		chatMsg, err := protocol.ParseChatMessage(packetRdr)
		if err != nil {
			slog.Warn("Failed to parse Chat Message", "error", err)
			return nil
		}
		s.eventBus.Publish(event.EventChat, event.NewChatEvent(ctx, connState.Username(), connState.UUID(), chatMsg.Message, event.SourcePlayerSend))
	}
	if tag == "C->S" && packet.ID == protocol.C2SChatCommand {
		packetRdr := bytes.NewReader(packet.Payload)
		chatCmd, err := protocol.ParseChatCommand(packetRdr)
		if err != nil {
			slog.Warn("Failed to parse Chat Command", "error", err)
			return nil
		}
		s.eventBus.Publish(event.EventChat, event.NewChatEvent(ctx, connState.Username(), connState.UUID(), chatCmd.Command, event.SourcePlayerCmd))

	}
	if tag == "C->S" && packet.ID == protocol.C2SChatCommandSigned {
		packetRdr := bytes.NewReader(packet.Payload)
		chatCmdSigned, err := protocol.ParseChatCommandSigned(packetRdr)
		if err != nil {
			slog.Warn("Failed to parse Chat Command Signed", "error", err)
			return nil
		}
		s.eventBus.Publish(event.EventChat, event.NewChatEvent(ctx, connState.Username(), connState.UUID(), chatCmdSigned.Command, event.SourcePlayerCmd))
	}
	return nil
}
