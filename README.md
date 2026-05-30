# DB Gateway — Multi-Tenant PostgreSQL Gateway (Go)

A high-performance TCP proxy that routes PostgreSQL connections to isolated per-tenant databases based on API key inspection. Serves as the single entry point for the Loonaris DBaaS platform.

## How It Works

```text
psql "postgres://sk_live_a3f9...64hex..._rw@db.loonaris.tech:5432/app"

  Gateway extracts user field from PostgreSQL startup packet
  → Parses sk_live_BASEKEY_MODE format
  → SHA256 hashes the base key
  → Looks up route in cache (60s TTL) or control plane API
  → Routes _rw to PgBouncer RW (Primary), _ro to PgBouncer RO (Replica)
  →Establishes bidirectional TCP tunnel
```

## Quick Start

```bash
# Build and run locally
go build -o db-gateway ./
PORT=5432 CONTROL_PLANE_URL=http://localhost:3001 INTERNAL_GATEWAY_SECRET=secret ./db-gateway

# Or run with Docker Compose (full test stack)
docker compose up --build
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `5432` | TCP listen port |
| `CONTROL_PLANE_URL` | `https://api.loonaris.internal` | Express backend URL for route lookups |
| `INTERNAL_GATEWAY_SECRET` | — | Bearer token for `/internal/routes` auth |

## Project Structure

```text
internal/
├── gateway/
│   ├── server.go        # TCP server, config, graceful shutdown
│   ├── session.go       # Connection handler: SSL/GSSENC, key parsing, read deadline
│   ├── tunnel.go        # RW/RO routing, bidirectional io.Copy tunnel
│   ├── key.go           # sk_live_[64hex]_[rw|ro] parser + SHA256 hashing
│   ├── cache.go         # sync.Map with 60s TTL for route caching
│   ├── api.go           # HTTP client for control plane route lookups
│   └── singleflight.go  # Deduplication for concurrent cache misses
├── postgres/
│   └── startup.go       # PostgreSQL startup packet + SSL/GSSENC handling
main.go                   # Entry point, env var config
```

## Testing

```bash
# Unit tests
go test ./... -v

# Docker integration test stack
docker compose up --build

# Connect as tenant 1 (RW)
psql "host=localhost port=35432 user=sk_live_aaaa...64a_rw dbname=app_test1"

# Connect as tenant 1 (RO)
psql "host=localhost port=35432 user=sk_live_aaaa...64a_ro dbname=app_test1"
```

## Documentation

- **[docs/GATEWAY_IMPL.md](docs/GATEWAY_IMPL.md)** — Implementation plan & file structure
- **[docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)** — EKS deployment, Kubernetes manifests, GitOps/CI-CD, architecture

## License

MIT