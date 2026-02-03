package proxy

import (
	"bytes"
	"errors"
	"io"
	"log"
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

func (s *Server) Start() error {
	log.Printf("[START] Starting proxy server on %s forwarding to %s", s.listenerAddr, s.backendAddr)
	netListener, err := net.Listen("tcp", s.listenerAddr)
	if err != nil {
		return err
	}
	defer netListener.Close()
	// Placeholder for accepting connections and proxying to backend
	for {
		conn, err := netListener.Accept()
		if err != nil {
			log.Printf("[ERROR] Error accepting connection: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(clientConn net.Conn) {
	backendConn, err := net.Dial("tcp", s.backendAddr)
	connState := &protocol.ConnState{}
	connState.Set(protocol.Handshaking)

	if err != nil {
		log.Printf("[ERROR] Error connecting to backend: %v", err)
		clientConn.Close()
		return
	}
	defer backendConn.Close()
	log.Printf("[PROXY] %s <-> %s", clientConn.RemoteAddr(), s.backendAddr)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := relayPackets(clientConn, backendConn, "C->S", connState)
		if err != nil {
			log.Printf("[ERROR] Error relaying packets C->S: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		err := relayPackets(backendConn, clientConn, "S->C", connState)
		if err != nil {
			log.Printf("[ERROR] Error relaying packets S->C: %v", err)
		}
	}()
	wg.Wait()
	log.Printf("[CLOSE] Connection closed: %s", clientConn.RemoteAddr())
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
				log.Printf("[HANDSHAKE] Protocol Version: %d, Server Address: %s, Server Port: %d, Next State: %d",
					handshake.ProtocolVersion, handshake.ServerAddress, handshake.ServerPort, handshake.NextState)
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
			log.Printf("[DEBUG LOGIN]%s Packet ID:0x%02X", tag, packet.ID)
			// Handle login state if needed
			if tag == "C->S" && packet.ID == 0x00 {
				// Example: Handle login start packet if needed
				packetRdr := bytes.NewReader(packet.Payload)
				loginStart, err := protocol.ParseLoginStart(packetRdr)
				if err != nil {
					return err
				}
				log.Printf("[LOGIN START] Username: %s UUID: %s", loginStart.Username, loginStart.UUID.String())
			}
			if tag == "S->C" && packet.ID == 0x02 {
				// Example: Handle login success packet if needed
				// After login success, switch to Play state
				log.Printf("[LOGIN SUCCESS] Switching to Play state")
				connState.Set(protocol.Play)
			}
		//case protocol.Play:
		// Handle play state if needed
		default:
			log.Printf("[%s]Packet ID:0x%02X", tag, packet.ID)
		}

		if err := protocol.WritePacket(dst, packet); err != nil {
			return err
		}
	}
}
