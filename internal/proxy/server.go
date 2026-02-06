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

	"github.com/Versifine/locus/internal/protocol"
)

type Server struct {
	listenerAddr string
	backendAddr  string
}

func NewServer(listenerAddr, backendAddr string) *Server {
	return &Server{listenerAddr: listenerAddr, backendAddr: backendAddr}
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
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(clientConn net.Conn) {
	// Disable Nagle's algorithm for lower latency
	if tcpConn, ok := clientConn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}

	backendConn, err := net.Dial("tcp", s.backendAddr)
	connState := protocol.NewConnState()
	connState.Set(protocol.Handshaking)

	if err != nil {
		slog.Error("Failed to connect backend", "backend", s.backendAddr, "error", err)
		clientConn.Close()
		return
	}

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
		err := relayPackets(clientConn, backendConn, "C->S", connState)
		if err != nil {
			slog.Error("Relay error", "dir", "C->S", "error", err)
		}
	}()
	go func() {
		defer wg.Done()
		err := relayPackets(backendConn, clientConn, "S->C", connState)
		if err != nil {
			slog.Error("Relay error", "dir", "S->C", "error", err)
		}
	}()
	wg.Wait()
	slog.Info("Connection closed", "client", clientConn.RemoteAddr())
}

func relayPackets(src, dst net.Conn, tag string, connState *protocol.ConnState) error {
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

		var newThreshold int = -2 // -2 means no change detected

		switch connState.Get() {
		case protocol.Handshaking:
			if tag == "C->S" && packet.ID == 0x00 {
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
		//case protocol.Status:
		// Handle status state if needed
		case protocol.Login:
			slog.Debug("Login packet", "dir", tag, "id", fmt.Sprintf("0x%02x", packet.ID))
			if tag == "C->S" && packet.ID == 0x00 {
				packetRdr := bytes.NewReader(packet.Payload)
				loginStart, err := protocol.ParseLoginStart(packetRdr)
				if err != nil {
					return err
				}
				slog.Info("Login start", "username", loginStart.Username, "uuid", loginStart.UUID.String())
			}
			if tag == "S->C" && packet.ID == 0x03 {
				packetRdr := bytes.NewReader(packet.Payload)
				threshold, err := protocol.ReadVarint(packetRdr)
				if err == nil {
					newThreshold = int(threshold)
					slog.Info("Compression enabled", "threshold", newThreshold)

				} else {
					slog.Error("Failed to parse compression threshold", "error", err)
				}
			}
			if tag == "S->C" && packet.ID == 0x02 {
				slog.Info("Login success, entering Configuration state")
				connState.Set(protocol.Configuration)
			}
		case protocol.Configuration:
			slog.Debug("Configuration packet", "dir", tag, "id", fmt.Sprintf("0x%02x", packet.ID))
			// S->C 0x03 = Finish Configuration
			if tag == "S->C" && packet.ID == 0x03 {
				slog.Info("Finish Configuration, entering Play state")
				connState.Set(protocol.Play)
			}
		case protocol.Play:
			slog.Debug("Play packet", "dir", tag, "id", fmt.Sprintf("0x%02x", packet.ID))
			if tag == "S->C" && packet.ID == 0x3f {
				packetRdr := bytes.NewReader(packet.Payload)
				playerChat, err := protocol.ParsePlayerChat(packetRdr)
				if err != nil {
					return err
				}
				slog.Info("Player chat message", "content", playerChat.PlainMessage, "timestamp", playerChat.Timestamp, "salt", playerChat.Salt)
			}
			if tag == "S->C" && packet.ID == 0x77 {
				packetRdr := bytes.NewReader(packet.Payload)
				systemChat, err := protocol.ParseSystemChat(packetRdr)
				if err != nil {
					return err
				}
				slog.Info("System chat", "content", protocol.FormatTextComponent(&systemChat.Content), "action_bar", systemChat.IsActionBar)
			}
			if tag == "C->S" && packet.ID == 0x08 {
				packetRdr := bytes.NewReader(packet.Payload)
				chatMsg, err := protocol.ParseChatMessage(packetRdr)
				if err != nil {
					return err
				}
				slog.Info("Chat message", "message", chatMsg.ChatMessage, "timestamp", chatMsg.Timestamp, "salt", chatMsg.Salt)
			}
			if tag == "C->S" && packet.ID == 0x06 {
				packetRdr := bytes.NewReader(packet.Payload)
				chatCmd, err := protocol.ParseChatCommand(packetRdr)
				if err != nil {
					return err
				}
				slog.Info("Chat command", "command", chatCmd.Command)
			}
			if tag == "C->S" && packet.ID == 0x07 {
				packetRdr := bytes.NewReader(packet.Payload)
				chatCmdSigned, err := protocol.ParseChatCommandSigned(packetRdr)
				if err != nil {
					return err
				}
				slog.Info("Signed chat command", "command", chatCmdSigned.Command)
			}
		default:
			slog.Debug("Packet", "dir", tag, "id", fmt.Sprintf("0x%02x", packet.ID))
		}

		// Write with current (OLD) threshold
		if err := protocol.WritePacket(dst, packet, currentThreshold); err != nil {
			return err
		}

		if newThreshold != -2 {
			connState.SetThreshold(newThreshold)
		}

	}
}
