<div align="center">

# 🛡️ DB Gateway

**Multi-tenant PostgreSQL gateway for the Loonaris DBaaS platform**

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-Wire%20Protocol-336791?logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](https://docs.docker.com/compose/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-EKS-326CE5?logo=kubernetes&logoColor=white)](https://kubernetes.io)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

</div>

---

A high-performance, multi-tenant PostgreSQL gateway written in Go. It acts as the single network entry point for the Loonaris DBaaS platform, terminating client authentication and transparently routing each connection to the correct per-tenant database based on the API key presented in the PostgreSQL startup packet.

## ✨ Features

- **Wire-protocol native** — speaks the PostgreSQL frontend/backend protocol directly; clients connect with any standard driver (`psql`, `pgx`, JDBC, ...).
- **API-key authentication** — the `user` field carries an `sk_live` key that doubles as the tenant credential. No per-user Postgres passwords are required.
- **Read/write splitting** — `_rw` keys route to the primary pooler, `_ro` keys to a read replica.
- **Auth-terminating proxy** — authenticates upstream to the tenant's PgBouncer as a shared internal user, then splices the two sockets together.
- **Route caching** — in-memory cache with a 60-second TTL and single-flight deduplication to shield the control plane from connection storms.
- **Graceful shutdown** — drains in-flight connections on `SIGINT`/`SIGTERM` within a configurable timeout.

## ⚙️ How It Works

```text
psql "postgres://sk_live_<64hex>_rw@db.loonaris.tech:5432/app"

  1. Read the PostgreSQL startup packet and extract the `user` field.
  2. Parse the sk_live_<basekey>_<mode> format and SHA-256 hash the base key.
  3. Resolve the tenant route from the local cache (60s TTL) or the control plane API.
  4. Route _rw → primary PgBouncer, _ro → replica PgBouncer.
  5. Authenticate to the pooler, synthesize the post-auth handshake to the client,
     and splice a bidirectional TCP tunnel.
```

## 🚀 Quick Start

```bash
# Build and run locally
go build -o db-gateway ./
PORT=5432 \
CONTROL_PLANE_URL=http://localhost:3001 \
INTERNAL_GATEWAY_SECRET=secret \
BACKEND_DB_USER=cloud_user \
BACKEND_DB_PASSWORD=changeme \
./db-gateway

# Or bring up the full integration stack
docker compose up --build
```

## 🔧 Configuration

All configuration is supplied through environment variables.

| Variable | Default | Description |
|---|---|---|
| `PORT` | `5432` | TCP port the gateway listens on. |
| `CONTROL_PLANE_URL` | `https://loonaris.tech/api` | Base URL of the control plane used for route lookups. |
| `INTERNAL_GATEWAY_SECRET` | — | Bearer token sent to the control plane's `/internal/routes` endpoint. |
| `BACKEND_DB_USER` | `cloud_user` | Shared internal user the gateway uses to authenticate to each tenant pooler. |
| `BACKEND_DB_PASSWORD` | — | Password for `BACKEND_DB_USER`. |

## 📁 Project Structure

```text
internal/
├── gateway/
│   ├── server.go        # TCP server, config, graceful shutdown
│   ├── session.go       # Connection handler: SSL/GSSENC negotiation, key parsing
│   ├── tunnel.go        # RW/RO routing, auth-terminating bidirectional tunnel
│   ├── key.go           # sk_live_<64hex>_<rw|ro> parser + SHA-256 hashing
│   ├── cache.go         # Route cache with 60s TTL
│   ├── api.go           # HTTP client for control plane route lookups
│   └── singleflight.go  # Deduplication for concurrent cache misses
└── postgres/
    └── startup.go       # PostgreSQL startup packet + SSL/GSSENC handling
main.go                  # Entry point and environment configuration
```

## 🧪 Testing

```bash
# Unit tests
go test ./... -v

# Integration stack
docker compose up --build

# Connect as a tenant (read/write)
psql "host=localhost port=35432 user=sk_live_<64hex>_rw dbname=app_test1"

# Connect as a tenant (read-only)
psql "host=localhost port=35432 user=sk_live_<64hex>_ro dbname=app_test1"
```

## 📄 License

Released under the [MIT License](LICENSE).
