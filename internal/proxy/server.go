package proxy

import (
	"bytes"
	"context"
	"errors"
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
	slog.Info("Starting proxy server", "listenerAddr", s.listenerAddr, "backendAddr", s.backendAddr)
	netListener, err := net.Listen("tcp", s.listenerAddr)
	if err != nil {
		return err
	}
	defer netListener.Close()
	go func() {
		<-ctx.Done()
		slog.Info("Shutting down proxy server")
		_ = netListener.Close()
	}()
	for {
		conn, err := netListener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				slog.Info("Proxy server stopped")
				return nil
			}
			slog.Error("Error accepting connection", "error", err)
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
	connState := &protocol.ConnState{}
	connState.Set(protocol.Handshaking)

	if err != nil {
		slog.Error("Error connecting to backend", "error", err)
		clientConn.Close()
		return
	}

	// Disable Nagle's algorithm for backend connection too
	if tcpConn, ok := backendConn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}

	defer backendConn.Close()
	slog.Info("Proxying connection", "client", clientConn.RemoteAddr(), "backend", s.backendAddr)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := relayPackets(clientConn, backendConn, "C->S", connState)
		if err != nil {
			slog.Error("Error relaying packets C->S", "error", err)
		}
	}()
	go func() {
		defer wg.Done()
		err := relayPackets(backendConn, clientConn, "S->C", connState)
		if err != nil {
			slog.Error("Error relaying packets S->C", "error", err)
		}
	}()
	wg.Wait()
	slog.Info("Connection closed", "client", clientConn.RemoteAddr())
}

func relayPackets(src, dst net.Conn, tag string, connState *protocol.ConnState) error {
	for {
		packet, err := protocol.ReadPacket(src)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch connState.Get() {
		case protocol.Handshaking:
			if tag == "C->S" && packet.ID == 0x00 {
				packetRdr := bytes.NewReader(packet.Payload)
				handshake, err := protocol.ParseHandshake(packetRdr)
				if err != nil {
					return err
				}
				slog.Info("Handshake", "ProtocolVersion", handshake.ProtocolVersion, "ServerAddress", handshake.ServerAddress, "ServerPort", handshake.ServerPort, "NextState", handshake.NextState)
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
			slog.Debug("Login state packet", "tag", tag, "packetID", packet.ID)
			// Handle login state if needed
			if tag == "C->S" && packet.ID == 0x00 {
				// Example: Handle login start packet if needed
				packetRdr := bytes.NewReader(packet.Payload)
				loginStart, err := protocol.ParseLoginStart(packetRdr)
				if err != nil {
					return err
				}
				slog.Info("Login start", "username", loginStart.Username, "uuid", loginStart.UUID.String())
			}
			if tag == "S->C" && packet.ID == 0x02 {
				// Example: Handle login success packet if needed
				// After login success, switch to Play state
				slog.Info("Login success, switching to Play state")
				connState.Set(protocol.Play)
			}
		//case protocol.Play:
		// Handle play state if needed
		default:
			slog.Debug("Packet received", "tag", tag, "packetID", packet.ID)
		}

		if err := protocol.WritePacket(dst, packet); err != nil {
			return err
		}
	}
}
