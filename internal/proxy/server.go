package proxy

import (
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
		err := relayPackets(clientConn, backendConn, "C->S")
		if err != nil {
			log.Printf("[ERROR] Error relaying packets C->S: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		err := relayPackets(backendConn, clientConn, "S->C")
		if err != nil {
			log.Printf("[ERROR] Error relaying packets S->C: %v", err)
		}
	}()
	wg.Wait()
	log.Printf("[CLOSE] Connection closed: %s", clientConn.RemoteAddr())
}

func relayPackets(src, dst net.Conn, tag string) error {
	for {
		packet, err := protocol.ReadPacket(src)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		log.Printf("[%s]Packet ID:0x%02X", tag, packet.ID)
		if err := protocol.WritePacket(dst, packet); err != nil {
			return err
		}
	}

}
