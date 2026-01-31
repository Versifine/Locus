package proxy

import (
	"fmt"
	"net"
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
		go func(c net.Conn) {
			defer c.Close()
			fmt.Println("Accepted connection from", c.RemoteAddr())
		}(conn)
	}
}
