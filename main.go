package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Loonaris-DBaaS/db-gateway/internal/gateway"
)

func main() {
	cfg := gateway.Config{
		Address:         fmt.Sprintf(":%d", envInt("PORT", 5432)),
		ControlPlaneURL: envStr("CONTROL_PLANE_URL", "https://loonaris.tech/api"),
		GatewaySecret:   envStr("INTERNAL_GATEWAY_SECRET", "static_shared_cluster_secret_token_here"),
	}

	server := gateway.NewServer(cfg)
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}