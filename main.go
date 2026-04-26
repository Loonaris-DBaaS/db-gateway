package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync/atomic"
)

var NUM_CLIENTS atomic.Int64

func main() {
	fmt.Println("Gateway listening on :5432")
	listener, err := net.Listen("tcp", ":5432")
	if err != nil {
		panic(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		NUM_CLIENTS.Add(1)
		go handleConn(conn)
	}
}
func handleConn(conn net.Conn) {
	defer conn.Close()
	fmt.Printf("Client number %d connected\n", NUM_CLIENTS.Load())

	var msgLen int32
	binary.Read(conn, binary.BigEndian, &msgLen)
	var version int32
	binary.Read(conn, binary.BigEndian, &version)

	body := make([]byte, msgLen-8)
	io.ReadFull(conn, body) //it keeps reading until the buffer(body is full) is full

	// body looks like: "user\0sk_live_abc\0database\0mydb\0\0"
	// bytes.Split on \0 gives: ["user", "sk_live_abc", "database", "mydb", "", ""]
	parts := bytes.Split(body, []byte{0})

	params := make(map[string]string)
	for i := 0; i+1 < len(parts); i += 2 {
		key := string(parts[i])
		val := string(parts[i+1])
		if key == "" {
			break // we hit the final \0\0 terminator here
		}
		params[key] = val
	}
	fmt.Printf("msgLen = %d\n", msgLen)
	fmt.Printf("version = %d\n", version)
	for k, v := range params {
		fmt.Printf(" %s = %s\n", k, v)
	}
	NUM_CLIENTS.Add(-1)
	fmt.Printf("Client disconnected %d left\n", NUM_CLIENTS.Load())
}
