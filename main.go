package main

import (
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
	fmt.Printf("msgLen = %d\n", msgLen)
	fmt.Printf("version = %d\n", version)
	fmt.Printf("%T\n", body)
	fmt.Printf("the rest = %s\n", body)

	NUM_CLIENTS.Add(-1)
	fmt.Printf("Client disconnected %d left\n", NUM_CLIENTS.Load())
}
