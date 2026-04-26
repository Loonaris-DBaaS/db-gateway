package gateway

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/Loonaris-DBaaS/db-gateway/internal/postgres"
)

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	defer func() {
		s.numClients.Add(-1)
		fmt.Printf("Client disconnected %d left\n", s.numClients.Load())
		fmt.Println("################")
	}()

	fmt.Printf("Client number %d connected\n", s.numClients.Load())

	startup, err := postgres.ReadStartup(conn)
	if err != nil {
		s.logConnError("read startup", err)
		return
	}

	fmt.Printf("msgLen = %d\n", startup.Length)
	fmt.Printf("version = %d\n", startup.Version)
	for k, v := range startup.Params {
		fmt.Printf(" %s = %s\n", k, v)
	}
}

func (s *Server) logConnError(step string, err error) {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		fmt.Printf("client closed during %s\n", step)
		return
	}
	fmt.Printf("%s failed: %v\n", step, err)
}
