# DB Gateway — Multi-Tenant PostgreSQL Gateway (Go)

A high-performance **Database Gateway system** written in Go that acts as the entry point for a multi-tenant Database-as-a-Service (DBaaS) platform.

It provides secure authentication, tenant routing, and transparent TCP proxying between external PostgreSQL clients and internal database infrastructure.

---

# What is this project?

This system is the **core networking layer of a DBaaS platform**, similar in concept to systems used by platforms like Supabase.

It solves the problem of:

> How do multiple external users securely connect to isolated PostgreSQL databases using a single public endpoint?

---

# Architecture


External Client (psql / app)
↓
DB Gateway (Go TCP Proxy)
↓
Authentication + Tenant Resolver
↓
PgBouncer (per tenant)
↓
CloudNative PostgreSQL Cluster


---

# Core Components

## 1. TCP Gateway (this service)
- Accepts PostgreSQL connections
- Reads startup packets
- Extracts authentication token
- Routes connection to correct backend

---

## 2. Authentication Layer
- API token-based authentication
- Token → Tenant mapping
- Secure validation before routing

---

## 3. Tenant Routing Engine
- Maps tenant → PgBouncer instance
- Supports multi-tenant isolation
- Dynamic backend resolution

---

## 4. Connection Proxy (Core Engine)
- Bidirectional TCP tunneling
- Zero-copy streaming using `io.Copy`
- Handles thousands of concurrent connections

---

## 5. PgBouncer Integration
- Connection pooling per tenant
- Reduces PostgreSQL load
- Improves scalability and performance

---

## 6. PostgreSQL Backend
- Managed via CloudNativePG
- Each tenant has isolated database cluster

---

# Authentication Flow

```
Client connects:
postgres://sk_live_token@db.gateway.com:5432
     ↓
Gateway extracts token from username
     ↓
Validates token in registry
     ↓
Resolves tenant
     ↓
Connects to PgBouncer
     ↓
Forwards traffic
```

## Connection Lifecycle

1. TCP connection established
2. Startup packet received
3. Token extracted
4. Tenant resolved
5. Backend selected
6. TCP tunnel established
7. Bidirectional streaming begins

---

# Getting Started

## Prerequisites

- Go 1.20+
- PostgreSQL or PgBouncer for testing

## Installation

Clone the repository:

```bash
git clone https://github.com/Loonaris-DBaaS/db-gateway.git
cd db-gateway
```

## Building

```bash
go build -o db-gateway ./
```

## Running

```bash
./db-gateway
```

The gateway listens on `0.0.0.0:5432` by default.

---

# Project Structure

```
.
├── main.go                           # Entry point
├── internal/
│   ├── gateway/
│   │   ├── server.go                # TCP server and connection handling
│   │   ├── session.go               # Session management
│   │   └── tunnel.go                # Bidirectional TCP tunneling
│   └── postgres/
│       └── startup.go               # PostgreSQL startup packet parsing
├── go.mod                           # Go module definition
└── README.md                        # This file
```

---

# Configuration

Currently, the gateway is configured via code. Future versions will support:

- Configuration files (YAML/TOML)
- Environment variables
- Dynamic backend resolution

---

# Development

### Running Tests

```bash
go test ./...
```

### Building Docker Image

```bash
docker build -t db-gateway .
```

---

# License

Licensed under the MIT License. See [LICENSE](./LICENSE) for details.
