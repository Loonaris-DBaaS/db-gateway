package gateway

import (
	"fmt"
	"net"
	"sync/atomic"
)

type Server struct {
	address    string
	numClients atomic.Int64
}

// we are using the constructor pattern here
func NewServer(address string) *Server {
	return &Server{address: address}
}

func (s *Server) ListenAndServe() error {
	fmt.Printf("Gateway listening on %s\n", s.address)
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("accept failed: %v\n", err)
			continue
		}

		s.numClients.Add(1)
		go s.handleConn(conn)
	}
}
