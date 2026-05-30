package gateway

import (
	"fmt"
	"io"
	"net"
	"time"
)

func (s *Server) tunnel(client net.Conn, startupRaw []byte, parsed *ParsedKey) error {
	route, err := s.lookupRoute(parsed.KeyHash)
	if err != nil {
		return fmt.Errorf("route lookup: %w", err)
	}

	if route.Status != "active" {
		return fmt.Errorf("tenant status=%s, expected active", route.Status)
	}

	var targetAddr string
	if parsed.Mode == "ro" {
		targetAddr = fmt.Sprintf("%s:%d", route.PgBouncerROHost, route.PgBouncerROPort)
	} else {
		targetAddr = fmt.Sprintf("%s:%d", route.PgBouncerRWHost, route.PgBouncerRWPort)
	}

	fmt.Printf("routing mode=%s -> %s\n", parsed.Mode, targetAddr)

	backend, err := net.DialTimeout("tcp", targetAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial backend %s: %w", targetAddr, err)
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
	<-done
	return nil
}