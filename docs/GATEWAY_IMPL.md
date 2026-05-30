# DB Gateway — Implementation Plan

This document specifies every code change required to transform the current skeleton
gateway into the production-grade routing proxy defined in `GATEWAY.md`. It covers
rw/ro mode selection, PgBouncer-separated routing, key parsing, caching, control plane
integration, and local Docker-based testing.

---

## 1. Current State vs Spec

| Capability              | Spec Requirement                                  | Current Code                         |
| ----------------------- | ------------------------------------------------- | ------------------------------------ |
| Key format validation   | `sk_live_[64hex]_[rw\|ro]`                        | None — raw string used as-is         |
| SHA256 base-key hashing | Hash base key for cache/API lookup                | None                                 |
| rw/ro mode routing      | Select `pgbouncer_rw_host` or `pgbouncer_ro_host` | None — single address per tenant     |
| Route caching           | `sync.Map` with 60s TTL                           | None — hardcoded `map[string]string` |
| Control plane API call  | `GET /internal/routes/{keyHash}` with Bearer auth | None                                 |
| Status guard            | Reject if status ≠ `"active"`                     | None                                 |
| SSL request handling    | Detect `80877103`, reply `'N'`                    | Implemented                          |
| Startup packet replay   | Forward unmodified raw bytes                      | Implemented                          |
| Bidirectional tunnel    | `io.Copy` x2                                      | Implemented                          |
| Configurable port       | Env var, default `5432`                           | Hardcoded `:5433`                    |

---

## 2. Target File Structure

```
db-gateway/
├── main.go                              # MODIFY — env vars, config struct
├── internal/
│   ├── gateway/
│   │   ├── server.go                    # MODIFY — Config struct, pass to session/tunnel
│   │   ├── session.go                   # MODIFY — key parsing, cache lookup
│   │   ├── tunnel.go                    # REWRITE — dynamic rw/ro routing
│   │   ├── key.go                       # NEW — ParseTenantKey + ParsedKey struct
│   │   ├── key_test.go                  # NEW — key parsing unit tests
│   │   ├── cache.go                     # NEW — sync.Map with TTL + TenantRoute
│   │   ├── cache_test.go                # NEW — cache TTL expiry unit tests
│   │   ├── api.go                       # NEW — HTTP client for control plane
│   │   └── api_test.go                  # NEW — HTTP client tests with httptest
│   └── postgres/
│       ├── startup.go                   # KEEP — already correct
│       └── startup_test.go              # NEW — startup packet + SSL edge cases
├── docker/
│   ├── stub-api/
│   │   ├── Dockerfile                   # NEW — Node.js stub control plane
│   │   ├── server.js                    # NEW — Express server returning fixture routes
│   │   ├── routes.json                  # NEW — 3 tenant fixture data
│   │   └── package.json                 # NEW — express dependency
│   ├── pgbouncer/
│   │   ├── pgbouncer-test1-rw.ini      # NEW — PgBouncer RW for tenant 1
│   │   ├── pgbouncer-test1-ro.ini      # NEW — PgBouncer RO for tenant 1
│   │   ├── pgbouncer-test2-rw.ini      # NEW — PgBouncer RW for tenant 2
│   │   ├── pgbouncer-test2-ro.ini      # NEW — PgBouncer RO for tenant 2
│   │   ├── pgbouncer-test3-rw.ini      # NEW — PgBouncer RW for tenant 3
│   │   ├── pgbouncer-test3-ro.ini      # NEW — PgBouncer RO for tenant 3
│   │   └── userlist.txt                 # NEW — PgBouncer user list for auth
│   └── init/
│       └── init.sql                     # REPLACE — unified init for 3 tenants
├── docker-compose.yml                   # REWRITE — full local test stack
├── go.mod
└── docs/
    └── GATEWAY_IMPL.md                  # THIS FILE
```

---

## 3. Test Tenant Keys

For local Docker testing, 3 tenants are pre-generated:

| Tenant | Base Key (64 hex chars) | Full RW Key          | Full RO Key          |
| ------ | ----------------------- | -------------------- | -------------------- |
| test1  | `aaa...a` (64 a's)      | `sk_live_aaa...a_rw` | `sk_live_aaa...a_ro` |
| test2  | `bbb...b` (64 b's)      | `sk_live_bbb...b_rw` | `sk_live_bbb...b_ro` |
| test3  | `ccc...c` (64 c's)      | `sk_live_ccc...c_rw` | `sk_live_ccc...c_ro` |

SHA256 hashes (used for cache/API lookup):

| Tenant | Key Hash                                                           |
| ------ | ------------------------------------------------------------------ |
| test1  | `ffe054fe7ae0cb6dc65c3af9b61d5209f439851db43d0ba5997337df154668eb` |
| test2  | `a0fab1377f49a759b57f63318262ebe89fabfc990e8e93ceac2984561482b9d4` |
| test3  | `52b6419d27bd7f547cee3b92f8c17a908b8a49601ecbec161e5030de1dfe9e0a` |

---

## 4. Step 1 — Key Parsing & Validation

**File:** `internal/gateway/key.go`

Validates the PostgreSQL `user` field against `sk_live_[64hex]_[rw|ro]`, extracts the
base key and mode, SHA256-hashes the base key for cache/API lookups.

On invalid keys, `ParseTenantKey` returns an error. The caller in `session.go` must
close the TCP connection silently (zero bytes returned) per GATEWAY.md Step 2.

---

## 5. Step 2 — Tenant Route Cache

**File:** `internal/gateway/cache.go`

Thread-safe `sync.Map` with 60-second TTL per entry. Stored as methods on `Server`
so the cache is lifecycle-managed with the server. On cache miss or TTL expiry,
the HTTP client (`api.go`) fetches from the control plane.

The `TenantRoute` struct matches the JSON schema from `GET /internal/routes/{keyHash}`:

```json
{
  "tenant_id": "project-test1",
  "pgbouncer_rw_host": "pgbouncer-test1-rw",
  "pgbouncer_rw_port": 5432,
  "pgbouncer_ro_host": "pgbouncer-test1-ro",
  "pgbouncer_ro_port": 5432,
  "status": "active"
}
```

---

## 6. Step 3 — Control Plane HTTP Client

**File:** `internal/gateway/api.go`

5-second timeout, no retries. Bearer token from `INTERNAL_GATEWAY_SECRET` env var.
Returns an error on 404 (invalid/revoked key) or non-200 status.

---

## 7. Step 4 — Rewritten Session & Tunnel

### session.go

- Calls `ParseTenantKey` on the `user` field
- On validation failure: `conn.Close()` immediately, zero bytes (per spec)
- On success: passes `*ParsedKey` to `tunnel()`

### tunnel.go

- Calls `s.lookupRoute(parsed.KeyHash)` which checks cache → API
- If `route.Status != "active"`: close connection
- Selects target based on `parsed.Mode`:
  - `"ro"` → `route.PgBouncerROHost:route.PgBouncerROPort`
  - `"rw"` → `route.PgBouncerRWHost:route.PgBouncerRWPort`
- Replays unmodified startup packet
- Bidirectional `io.Copy` tunnel

The hardcoded `tenantDatabases` map is **deleted**.

---

## 8. Step 5 — Server Configuration

**File:** `main.go`

Environment variables:

- `PORT` — default `5432`
- `CONTROL_PLANE_URL` — default `https://api.loonaris.internal`
- `INTERNAL_GATEWAY_SECRET` — default `static_shared_cluster_secret_token_here`

All are passed via a `Config` struct to `NewServer()`.

---

## 9. Step 6 — Unit Tests

All tests use Go stdlib (`testing`, `net/http/httptest`, `net.Pipe()`):

- `key_test.go` — Table-driven tests for valid/invalid key formats
- `cache_test.go` — TTL expiry, cache miss → API fetch, cache hit with fresh/stale
- `api_test.go` — `httptest.NewServer` stub returning fixture JSON, 404, timeouts
- `startup_test.go` — `net.Pipe()` to simulate SSL request + startup packet parsing

---

## 10. Local Docker Testing

### Architecture

```
psql client
    │
    ▼ :5432
┌──────────────────┐
│  db-gateway       │  (Go binary)
└────────┬─────────┘
         │ GET /internal/routes/{hash}
         ▼ :3001
┌──────────────────┐
│  stub-api         │  (Node.js — returns fixture routes for 3 tenants)
└──────────────────┘
         │
         │ Gateway dials PgBouncer services via Docker network
         ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│ pgbouncer-test1 │  │ pgbouncer-test2 │  │ pgbouncer-test3 │
│    -rw  :-ro    │  │    -rw  :-ro    │  │    -rw  :-ro    │
└────────┬────────┘  └────────┬────────┘  └────────┬────────┘
         │                    │                    │
         └────────────────────┼────────────────────┘
                              ▼ :5432
                    ┌──────────────────┐
                    │  postgres        │  (databases: app_test1, app_test2, app_test3)
                    └──────────────────┘
```

### Connecting

```bash
# RW key for tenant 1
psql "postgres://tenant1_example_rw@localhost:5432/app_test1"

# RO key for tenant 1
psql "postgres://tenant1_example_ro@localhost:5432/app_test1"

# Invalid key (should be silently dropped)
psql "postgres://invalid_user@localhost:5432/testdb"
```

### Verifying rw/ro routing

Check gateway logs to confirm:

- RW key → dials `pgbouncer-test1-rw:5432`
- RO key → dials `pgbouncer-test1-ro:5432`
- Cache hit on 2nd connection (no stub-api log line)
- Cache expiry after 60s (new stub-api request)

---

## 11. Dependencies

Zero external Go dependencies. All new code uses only the standard library.

`go.mod` remains at `go 1.25.5` with no new `require` lines.
