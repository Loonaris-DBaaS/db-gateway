package main

import (
	"fmt"
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
	defer fmt.Println("Client closed connection")
	defer conn.Close()
	fmt.Printf("Client number %d connected\n", NUM_CLIENTS.Load())
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			break // client disconnected
		}
		fmt.Printf("got %d bytes\n", n)
	}
	NUM_CLIENTS.Add(-1)
	fmt.Printf("Client disconnected %d left\n", NUM_CLIENTS.Load())
}
