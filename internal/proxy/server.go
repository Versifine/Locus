package proxy

import (
	"io"
	"log"
	"net"
	"sync"
)

type Server struct {
	listenerAddr string
	backendAddr  string
}

func NewServer(listenerAddr, backendAddr string) *Server {
	return &Server{listenerAddr: listenerAddr, backendAddr: backendAddr}
}

func (s *Server) Start() error {
	log.Printf("Starting proxy server on %s forwarding to %s", s.listenerAddr, s.backendAddr)
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
	copyAndClose := func(dst, src net.Conn) {
		defer wg.Done()
		_, err := io.Copy(dst, src)
		if err != nil {
			log.Printf("[ERROR] Error during data copy: %v", err)
		}

		dst.Close()
		src.Close()
	}
	go copyAndClose(backendConn, clientConn)
	go copyAndClose(clientConn, backendConn)
	wg.Wait()
	log.Printf("[CLOSE] Connection closed: %s", clientConn.RemoteAddr())
}
