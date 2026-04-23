FULL README.md (final project)
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
postgres://sk\_live\_token@db.gateway.com:5432

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

Connection Lifecycle
1- TCP connection established
2- Startup packet received
3- Token extracted
4- Tenant resolved
5- Backend selected
6- TCP tunnel established
7- Bidirectional streaming begins