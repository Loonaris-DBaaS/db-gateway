package postgres

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
)

func TestReadStartup_WithoutSSL(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		msg := buildStartupMessage(t, "sk_live_aaab_rw", "testdb")
		_, _ = client.Write(msg)
	}()

	msg, err := ReadStartup(server)
	if err != nil {
		t.Fatalf("ReadStartup: %v", err)
	}
	if msg.Params["user"] != "sk_live_aaab_rw" {
		t.Errorf("user=%s, want sk_live_aaab_rw", msg.Params["user"])
	}
	if msg.Params["database"] != "testdb" {
		t.Errorf("database=%s, want testdb", msg.Params["database"])
	}
}

func TestReadStartup_WithSSLRequest(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = binary.Write(client, binary.BigEndian, int32(8))
		_ = binary.Write(client, binary.BigEndian, int32(80877103))

		buf := make([]byte, 1)
		_, _ = client.Read(buf)
		if buf[0] != 'N' {
			t.Errorf("expected 'N' response to SSLRequest, got %c", buf[0])
		}

		msg := buildStartupMessage(t, "sk_live_bbbb_ro", "mydb")
		_, _ = client.Write(msg)
	}()

	msg, err := ReadStartup(server)
	if err != nil {
		t.Fatalf("ReadStartup with SSL: %v", err)
	}
	if msg.Params["user"] != "sk_live_bbbb_ro" {
		t.Errorf("user=%s, want sk_live_bbbb_ro", msg.Params["user"])
	}
}

func TestReadStartup_WithGSSENCRequest(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = binary.Write(client, binary.BigEndian, int32(8))
		_ = binary.Write(client, binary.BigEndian, int32(80877104))

		buf := make([]byte, 1)
		_, _ = client.Read(buf)
		if buf[0] != 'N' {
			t.Errorf("expected 'N' response to GSSENCRequest, got %c", buf[0])
		}

		msg := buildStartupMessage(t, "sk_live_cccc_rw", "gssdb")
		_, _ = client.Write(msg)
	}()

	msg, err := ReadStartup(server)
	if err != nil {
		t.Fatalf("ReadStartup with GSSENC: %v", err)
	}
	if msg.Params["user"] != "sk_live_cccc_rw" {
		t.Errorf("user=%s, want sk_live_cccc_rw", msg.Params["user"])
	}
}

func TestReadStartup_UnknownRequestCode(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = binary.Write(client, binary.BigEndian, int32(8))
		_ = binary.Write(client, binary.BigEndian, int32(12345))
	}()

	_, err := ReadStartup(server)
	if err == nil {
		t.Fatal("expected error for unknown request code, got nil")
	}
}

func TestReadStartup_InvalidLength(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = binary.Write(client, binary.BigEndian, int32(4))
		_, _ = client.Write([]byte{0, 0, 0, 0})
	}()

	_, err := ReadStartup(server)
	if err == nil {
		t.Fatal("expected error for invalid startup length, got nil")
	}
}

func buildStartupMessage(t *testing.T, user, database string) []byte {
	t.Helper()
	body := []byte("user\000" + user + "\000database\000" + database + "\000\000")

	length := int32(4 + 4 + len(body))
	msg := make([]byte, 4+4+len(body))
	binary.BigEndian.PutUint32(msg[0:4], uint32(length))
	binary.BigEndian.PutUint32(msg[4:8], uint32(196608))
	copy(msg[8:], body)
	return msg
}

func TestReadStartup_RawPreservation(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		msg := buildStartupMessage(t, "sk_live_test_rw", "db1")
		_, _ = client.Write(msg)
		_ = client.Close()
	}()

	msg, err := ReadStartup(server)
	if err != nil {
		t.Fatalf("ReadStartup: %v", err)
	}

	if len(msg.Raw) == 0 {
		t.Fatal("Raw field should not be empty")
	}

	binary.BigEndian.PutUint32(msg.Raw[0:4], uint32(msg.Length))

	expectedLen := 4 + 4 + len([]byte("user\000sk_live_test_rw\000database\000db1\000\000"))
	if int(msg.Length) != expectedLen {
		t.Errorf("Raw length=%d, expected=%d", msg.Length, expectedLen)
	}

	_, _ = io.ReadAll(server)
}