package main

import (
	"fmt"
	"net"
)

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
		fmt.Println("New Client Connected")

		conn.Close()
	}
}
