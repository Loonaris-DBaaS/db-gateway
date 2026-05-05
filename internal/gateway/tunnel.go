package gateway

import (
	"fmt"
	"io"
	"net"
)

var tenantDatabases = map[string]string{
	"sk_live_tenant1": "localhost:5434",
	"sk_live_tenant2": "localhost:5435",
	"sk_live_tenant3": "localhost:5436",
}

func (s *Server) tunnel(client net.Conn, startupRaw []byte, tenantKey string) error {
	addr, exists := tenantDatabases[tenantKey]
	if !exists {
		return fmt.Errorf("tenant not found: %s", tenantKey)
	}
	backend, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to dial backend for tenant %s: %w", tenantKey, err)
	}
	defer backend.Close()

	if _, err := backend.Write(startupRaw); err != nil {
		return fmt.Errorf("forward startup packet: %w", err)
	}

	done := make(chan struct{}, 2)

	go func() {
		io.Copy(backend, client)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(client, backend)
		done <- struct{}{}
	}()

	<-done
	return nil
}