package gateway

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/Loonaris-DBaaS/db-gateway/internal/postgres"
)

const readDeadline = 30 * time.Second

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	defer func() {
		s.numClients.Add(-1)
		fmt.Printf("Client disconnected, %d remaining\n", s.numClients.Load())
	}()

	fmt.Printf("Client connected, total %d, remote=%s\n", s.numClients.Load(), conn.RemoteAddr())

	if err := conn.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
		fmt.Printf("set read deadline: %v\n", err)
		return
	}

	startup, err := postgres.ReadStartup(conn)
	if err != nil {
		s.logConnError("read startup", err)
		return
	}

	tenantKey := startup.Params["user"]
	if tenantKey == "" {
		fmt.Printf("missing user field in startup packet\n")
		return
	}

	parsed, err := ParseTenantKey(tenantKey)
	if err != nil {
		fmt.Printf("invalid key format from remote=%s\n", conn.RemoteAddr())
		return
	}

	fmt.Printf("key_hash=%s... mode=%s database=%s remote=%s\n",
		parsed.KeyHash[:12], parsed.Mode, startup.Params["database"], conn.RemoteAddr())

	conn.SetDeadline(time.Time{})

	if err := s.tunnel(conn, startup.Raw, parsed); err != nil {
		s.logConnError("tunnel", err)
	}
}

func (s *Server) logConnError(step string, err error) {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		fmt.Printf("client closed during %s\n", step)
		return
	}
	fmt.Printf("%s failed: %v\n", step, err)
}