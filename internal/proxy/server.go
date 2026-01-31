package proxy

import (
	"fmt"
	"io"
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
	fmt.Println("Starting proxy server on", s.listenerAddr, "forwarding to", s.backendAddr)
	netListener, err := net.Listen("tcp", s.listenerAddr)
	if err != nil {
		return err
	}
	defer netListener.Close()
	// Placeholder for accepting connections and proxying to backend
	for {
		conn, err := netListener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(clientConn net.Conn) {
	backendConn, err := net.Dial("tcp", s.backendAddr)
	if err != nil {
		fmt.Println("Error connecting to backend:", err)
		clientConn.Close()
		return
	}
	defer backendConn.Close()
	fmt.Println("Proxying data between", clientConn.RemoteAddr(), "and backend", s.backendAddr)
	var wg sync.WaitGroup
	wg.Add(2)
	copyAndClose := func(dst, src net.Conn) {
		defer wg.Done()
		_, err := io.Copy(dst, src)
		if err != nil {
			fmt.Println("Error during data copy:", err)
		}

		dst.Close()
		src.Close()
	}
	go copyAndClose(backendConn, clientConn)
	go copyAndClose(clientConn, backendConn)
	wg.Wait()
	fmt.Println("Finished proxying for", clientConn.RemoteAddr())
}
