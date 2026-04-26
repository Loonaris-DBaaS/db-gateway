package postgres

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

const sslRequestCode int32 = 80877103

type StartupMessage struct {
	Length  int32
	Version int32
	Params  map[string]string
}

func ReadStartup(conn net.Conn) (*StartupMessage, error) {
	msgLen, err := readInt32(conn)
	if err != nil {
		return nil, fmt.Errorf("read message length: %w", err)
	}

	if msgLen == 8 {
		if err := handleSSLRequest(conn); err != nil {
			return nil, err
		}

		msgLen, err = readInt32(conn)
		if err != nil {
			return nil, fmt.Errorf("read startup message length: %w", err)
		}
	}

	if msgLen < 8 {
		return nil, fmt.Errorf("invalid startup length: %d", msgLen)
	}

	version, err := readInt32(conn)
	if err != nil {
		return nil, fmt.Errorf("read startup version: %w", err)
	}

	body := make([]byte, msgLen-8)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, fmt.Errorf("read startup body: %w", err)
	}

	return &StartupMessage{
		Length:  msgLen,
		Version: version,
		Params:  parseParams(body),
	}, nil
}

func handleSSLRequest(conn net.Conn) error {
	code, err := readInt32(conn)
	if err != nil {
		return fmt.Errorf("read SSL request code: %w", err)
	}

	if code == sslRequestCode {
		if _, err := conn.Write([]byte{'N'}); err != nil {
			return fmt.Errorf("write SSL rejection: %w", err)
		}
	}

	return nil
}

func readInt32(conn net.Conn) (int32, error) {
	var v int32
	if err := binary.Read(conn, binary.BigEndian, &v); err != nil {
		return 0, err
	}
	return v, nil
}

func parseParams(body []byte) map[string]string {
	parts := bytes.Split(body, []byte{0})
	params := make(map[string]string)

	for i := 0; i+1 < len(parts); i += 2 {
		key := string(parts[i])
		val := string(parts[i+1])
		if key == "" {
			break
		}
		params[key] = val
	}

	return params
}

