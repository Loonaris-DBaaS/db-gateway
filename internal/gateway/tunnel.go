package gateway

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
)

const backendConnectTimeout = 10 * time.Second

// tunnel is an auth-terminating proxy. The client authenticated to us by
// presenting a valid sk_live key in the startup `user` field (the key IS the
// credential) — it has no real Postgres password. So we open our own
// authenticated connection to the tenant's pooler as the shared internal DB
// user, then splice the two sockets together.
func (s *Server) tunnel(client net.Conn, parsed *ParsedKey, database string) error {
	route, err := s.lookupRoute(parsed.KeyHash)
	if err != nil {
		return fmt.Errorf("route lookup: %w", err)
	}

	if route.Status != "active" {
		return fmt.Errorf("tenant status=%s, expected active", route.Status)
	}

	var host string
	var port int
	if parsed.Mode == "ro" {
		host, port = route.PgBouncerROHost, route.PgBouncerROPort
	} else {
		host, port = route.PgBouncerRWHost, route.PgBouncerRWPort
	}

	if database == "" {
		database = "app"
	}

	fmt.Printf("routing mode=%s -> %s:%d db=%s\n", parsed.Mode, host, port, database)

	// Connect + authenticate to the pooler as the shared internal user.
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, s.config.BackendDBUser, s.config.BackendDBPassword, database,
	)
	ctx, cancel := context.WithTimeout(context.Background(), backendConnectTimeout)
	defer cancel()

	pgConn, err := pgconn.Connect(ctx, connStr)
	if err != nil {
		return fmt.Errorf("backend connect %s:%d: %w", host, port, err)
	}

	// Take over the raw, authenticated socket. After Hijack the connection is
	// past authentication and sitting at ReadyForQuery.
	hijacked, err := pgConn.Hijack()
	if err != nil {
		return fmt.Errorf("hijack backend conn: %w", err)
	}
	backend := hijacked.Conn
	defer backend.Close()

	// Synthesize the post-auth handshake for the client: tell it auth succeeded
	// and replay the server parameters / key data we received from the backend.
	var buf []byte
	if buf, err = (&pgproto3.AuthenticationOk{}).Encode(buf); err != nil {
		return fmt.Errorf("encode AuthenticationOk: %w", err)
	}
	for name, value := range hijacked.ParameterStatuses {
		if buf, err = (&pgproto3.ParameterStatus{Name: name, Value: value}).Encode(buf); err != nil {
			return fmt.Errorf("encode ParameterStatus: %w", err)
		}
	}
	if buf, err = (&pgproto3.BackendKeyData{ProcessID: hijacked.PID, SecretKey: hijacked.SecretKey}).Encode(buf); err != nil {
		return fmt.Errorf("encode BackendKeyData: %w", err)
	}
	if buf, err = (&pgproto3.ReadyForQuery{TxStatus: 'I'}).Encode(buf); err != nil {
		return fmt.Errorf("encode ReadyForQuery: %w", err)
	}
	if _, err := client.Write(buf); err != nil {
		return fmt.Errorf("write client handshake: %w", err)
	}

	// Splice. Closing both conns when either direction finishes unblocks the
	// other io.Copy; otherwise a half-closed peer leaves one copy (and this
	// goroutine + the FDs) blocked forever.
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(backend, client)
		backend.Close()
		client.Close()
		done <- struct{}{}
	}()
	go func() {
		io.Copy(client, backend)
		backend.Close()
		client.Close()
		done <- struct{}{}
	}()

	<-done
	<-done
	return nil
}
