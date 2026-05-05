package main

import "github.com/Loonaris-DBaaS/db-gateway/internal/gateway"

func main() {
	server := gateway.NewServer(":5433")
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}
