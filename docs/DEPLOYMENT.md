# DB Gateway — EKS Deployment & Architecture Guide

This document specifies how the DB Gateway maps to the production EKS architecture,
how it gets deployed via GitOps/CI-CD, and the complete set of Kubernetes manifests
required to run it alongside tenant compute planes.

The Express Backend (Control Plane) runs **outside** EKS on AWS ECS Fargate.
The DB Gateway runs **inside** EKS and reaches the control plane via the ALB endpoint.

---

## 1. Production Architecture Map

```text
                              INTERNET
                                 │
                                 ▼ :5432
                    ┌────────────────────────────┐
                    │   AWS Network Load Balancer  │
                    │   (NLB — TCP, port 5432)     │
                    └────────────┬─────────────────┘
                                 │
        ═══════════════════════════════════════════════════════
        ║                  AWS EKS CLUSTER                      ║
        ║                                                      ║
        ║  NODE 1: SYSTEM PLANE (Untainted)                    ║
        ║  ┌──────────────────────────────────────────────────┐ ║
        ║  │  db-gateway Pod                                  │ ║
        ║  │  Listens :5432                                    │ ║
        ║  │  Env: CONTROL_PLANE_URL ──────────────────────┐  │ ║
        ║  │  Env: INTERNAL_GATEWAY_SECRET                  │  │ ║
        ║  └─────────────┬─────────────────────────────────┘  ║
        ║                │                                     ║
        ║                │ Cache HIT → use cached route        ║
        ║                │ Cache MISS ─────────────────────────┼──┼──► https://loonaris.tech/api
        ║                │    GET /api/internal/routes/{hash}  │  │     │
        ║                │    Authorization: Bearer <secret>   │  │     ▼
        ║                │                                    │  │  ┌──────────────────┐
        ║                │                                    │  │  │ Nginx EC2        │
        ║                │                                    │  │  │ (SSL termination) │
        ║                │                                    │  │  └────────┬─────────┘
        ║                │                                    │  │           │ /api/*
        ║                │                                    │  │           ▼
        ║                │                                    │  │  ┌──────────────────┐
        ║                │                                    │  │  │ ALB → ECS Fargate │
        ║                │                                    │  │  │ Express Backend   │
        ║                │                                    │  │  └──────────────────┘
        ║                │                                    │  │
        ║  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─  │  │
        ║  Internal K8s DNS resolution                       │  │
        ║                │                                    │  │
        ║                ▼                                    │  │
        ║  NODES 2-4: TENANT PLANE (Tainted)                 │  │
        ║  ┌──────────────────────────────────────────────────┐ ║
        ║  │  Namespace: project-abc12345                       │ ║
        ║  │                                                    │ ║
        ║  │  ┌─────────────────┐    ┌─────────────────┐       │ ║
        ║  │  │ pgbouncer-rw    │    │ pgbouncer-ro    │       │ ║
        ║  │  │ Deployment      │    │ Deployment      │       │ ║
        ║  │  │   :5432         │    │   :5432         │       │ ║
        ║  │  └────────┬────────┘    └────────┬────────┘       │ ║
        ║  │           │                      │                 │ ║
        ║  │  ┌────────┴────────┐    ┌────────┴────────┐       │ ║
        ║  │  │ pooler-rw-svc   │    │ pooler-ro-svc   │       │ ║
        ║  │  │ ClusterIP:5432  │    │ ClusterIP:5432  │       │ ║
        ║  │  └────────┬────────┘    └────────┬────────┘       │ ║
        ║  │           │                      │                 │ ║
        ║  │  ┌────────┴────────────────────────┴────────┐       │ ║
        ║  │  │  CNPG Cluster: instance-db                │       │ ║
        ║  │  │  Primary (rw) ──stream──▶ Replica (ro)   │       │ ║
        ║  │  └───────────────────────────────────────── ─┘       │ ║
        ║  └──────────────────────────────────────────────────┘  ║
        ║                                                      ║
        ═══════════════════════════════════════════════════════
```

### Request Flow (RW Key Example)

```text
psql "postgres://sk_live_a3f9...64hex..._rw@db.loonaris.tech:5432/app"

  1. NLB forwards TCP to db-gateway Pod :5432
  2. Gateway reads PostgreSQL startup packet
  3. Extracts user="sk_live_a3f9...64hex..._rw"
  4. ParseTenantKey → baseKey="a3f9...", mode="rw", keyHash=SHA256("a3f9...")
  5. lookupRoute(keyHash)
     a. Cache HIT → use cached TenantRoute
     b. Cache MISS → GET https://loonaris.tech/api
                      /internal/routes/{keyHash}
                      Authorization: Bearer <INTERNAL_GATEWAY_SECRET>
  6. TenantRoute.Status == "active" → proceed
  7. mode=="rw" → target = pooler-rw-svc.project-abc12345.svc.cluster.local:5432
  8. net.DialTimeout("tcp", target, 10s)
  9. Forward unmodified startup packet to PgBouncer
  10. Bidirectional io.Copy tunnel: Client ↔ Gateway ↔ PgBouncer ↔ CNPG Primary
```

### Request Flow (RO Key — Simultaneous, Separate Connection)

```text
psql "postgres://sk_live_a3f9...64hex..._ro@db.loonaris.tech:5432/app"

  Same steps 1-6, then:
  7. mode=="ro" → target = pooler-ro-svc.project-abc12345.svc.cluster.local:5432
  8. Routes to PgBouncer RO → CNPG Replica
  9. Bidirectional pipe: Client ↔ Gateway ↔ PgBouncer RO ↔ CNPG Replica
```

Both RW and RO connections run simultaneously as independent goroutines.
The gateway is a transparent pipe after handshake — zero protocol awareness.

---

## 2. Environment Variables

| Variable | Required | Default | Production Value |
|---|---|---|---|
| `PORT` | No | `5432` | `5432` |
| `CONTROL_PLANE_URL` | Yes | `https://api.loonaris.internal` | `https://loonaris.tech/api` |
| `INTERNAL_GATEWAY_SECRET` | Yes | — | Generated via `openssl rand -hex 32` |

The Express backend is **external to EKS** (runs on ECS Fargate), reachable via
the Nginx reverse proxy at `https://loonaris.tech/api`. The gateway calls this
public endpoint which routes through the Nginx EC2 instance (SSL termination) to
the ALB and then to ECS. Since the Nginx EC2 and EKS cluster are in the same VPC,
this traffic stays on the AWS internal network.

---

## 3. Key Parsing & Routing Logic (Production Reference)

### Key Format
```text
sk_live_[64-char-hex]_[rw|ro]
         │                │
         └── Base Key ────┘── Mode Suffix
```

### Processing Pipeline
1. **Parse** `user` field from PostgreSQL startup packet: `^sk_live_([a-f0-9]{64})_(rw|ro)$`
2. **Extract** `baseKey` (group 1) and `mode` (group 2)
3. **Hash** `SHA256(baseKey)` → `keyHash` (64-char hex string)
4. **Lookup** `keyHash` in local `sync.Map` cache (60s TTL, singleflight deduplication)
5. **Fallback** `GET {CONTROL_PLANE_URL}/api/internal/routes/{keyHash}` with Bearer auth
6. **Guard** Reject if `status != "active"` — drop connection silently
7. **Route** `mode=="rw"` → `pgbouncer_rw_host:pgbouncer_rw_port`, `mode=="ro"` → `pgbouncer_ro_host:pgbouncer_ro_port`
8. **Tunnel** Replay unmodified startup packet, then bidirectional `io.Copy`

### Security Properties
- Plaintext `baseKey` is **never** sent to the control plane — only the SHA256 hash
- Invalid key format → silent connection drop (zero bytes returned)
- Revoked/expired keys → control plane returns 404 → connection drop
- Non-active tenants (`provisioning`, `suspended`) → connection drop
- Read deadline of 30s on startup packet prevents Slowloris attacks
- GSSENCRequest (code 80877104) is rejected with `'N'` alongside SSLRequest

---

## 4. GitOps / CI-CD Pipeline

### 4.1 GitHub Actions Workflow (`.github/workflows/docker.yml`)

The pipeline:
1. Runs `go test` and `go vet`
2. Builds the Docker image for `linux/amd64` and `linux/arm64`
3. Pushes to Docker Hub with `latest`, `sha-<short>`, and version tags
4. On main pushes: updates the GitOps manifest repo to trigger ArgoCD sync
5. On version tags: deploys to production with manual approval

The full workflow is in `.github/workflows/docker.yml`.

### 4.2 Image Tagging Strategy

| Event | Tag | Deployment |
|---|---|---|
| Push to `main` | `latest`, `sha-abc1234` | Auto → staging |
| Tag `v1.2.3` | `1.2.3`, `1.2`, `latest` | Manual approval → production |

### 4.3 ArgoCD Application

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: db-gateway
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/Loonaris-DBaaS/k8s-manifests.git
    targetRevision: main
    path: infra/db-gateway
  destination:
    server: https://kubernetes.default.svc
    namespace: system-plane
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

---

## 5. Kubernetes Manifests

All manifests are in `k8s/` in this repository and also referenced from the
`k8s-manifests` GitOps repository for ArgoCD.

### 5.1 Namespace

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: system-plane
  labels:
    platform.loonaris.tech/system: "true"
```

### 5.2 Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: db-gateway
  namespace: system-plane
  labels:
    app: db-gateway
spec:
  replicas: 2
  selector:
    matchLabels:
      app: db-gateway
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  template:
    metadata:
      labels:
        app: db-gateway
    spec:
      tolerations:
        - key: "dedicated"
          operator: "Equal"
          value: "system"
          effect: "NoSchedule"
      nodeSelector:
        node-role.loonaris.tech/system: "true"
      containers:
        - name: db-gateway
          image: dockerhub-username/db-gateway:latest
          imagePullPolicy: Always
          ports:
            - containerPort: 5432
              protocol: TCP
          env:
            - name: PORT
              value: "5432"
            - name: CONTROL_PLANE_URL
              valueFrom:
                secretKeyRef:
                  name: db-gateway-secrets
                  key: control-plane-url
            - name: INTERNAL_GATEWAY_SECRET
              valueFrom:
                secretKeyRef:
                  name: db-gateway-secrets
                  key: gateway-secret
          resources:
            requests:
              cpu: 100m
              memory: 64Mi
            limits:
              cpu: 500m
              memory: 256Mi
          readinessProbe:
            tcpSocket:
              port: 5432
            initialDelaySeconds: 3
            periodSeconds: 10
          livenessProbe:
            tcpSocket:
              port: 5432
            initialDelaySeconds: 5
            periodSeconds: 30
          lifecycle:
            preStop:
              exec:
                command: ["sh", "-c", "sleep 5"]
      terminationGracePeriodSeconds: 45
```

### 5.3 Service (ClusterIP — internal)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: db-gateway
  namespace: system-plane
  labels:
    app: db-gateway
spec:
  type: ClusterIP
  ports:
    - port: 5432
      targetPort: 5432
      protocol: TCP
  selector:
    app: db-gateway
```

### 5.4 NLB Service (exposes gateway to the internet)

The AWS NLB terminates TCP on port 5432 and forwards to the gateway pods.
Using the AWS Load Balancer Controller:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: db-gateway-nlb
  namespace: system-plane
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: nlb
    service.beta.kubernetes.io/aws-load-balancer-scheme: internet-facing
    service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled: "true"
  labels:
    app: db-gateway
spec:
  type: LoadBalancer
  ports:
    - port: 5432
      targetPort: 5432
      protocol: TCP
  selector:
    app: db-gateway
```

### 5.5 Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: db-gateway-secrets
  namespace: system-plane
type: Opaque
stringData:
  # Routes through Nginx EC2 (SSL termination) → ALB → ECS Express Backend
  control-plane-url: "https://loonaris.tech/api"
  # Shared secret — must match INTERNAL_GATEWAY_SECRET env var on the ECS task definition
  gateway-secret: "REPLACE_WITH_GENERATED_SECRET"
```

Generate the shared secret:
```bash
openssl rand -hex 32
```

The same value must be set as `INTERNAL_GATEWAY_SECRET` environment variable on the
ECS task definition for the Express backend. The Nginx EC2 at `loonaris.tech`
proxies `/api/*` to the ALB, so the gateway uses `https://loonaris.tech/api` as its
control plane URL — keeping routing consistent with the frontend.

### 5.6 NetworkPolicy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: db-gateway
  namespace: system-plane
spec:
  podSelector:
    matchLabels:
      app: db-gateway
  policyTypes:
    - Ingress
    - Egress
  ingress:
    # Allow NLB health checks and client connections
    - from: []
      ports:
        - port: 5432
          protocol: TCP
  egress:
    # Allow DNS resolution
    - to: []
      ports:
        - port: 53
          protocol: UDP
        - port: 53
          protocol: TCP
    # Allow egress to the Express backend ALB (outside EKS)
    # This resolves via AWS VPC DNS to the ALB IP addresses
    - to: []
      ports:
        - port: 80
          protocol: TCP
        - port: 443
          protocol: TCP
    # Allow egress to tenant namespaces for PgBouncer connections
    - to:
        - namespaceSelector:
            matchLabels:
              platform.loonaris.tech/tenant: "true"
      ports:
        - port: 5432
          protocol: TCP
```

---

## 6. DNS Configuration

The gateway is exposed to the public internet via a custom domain:

```text
db.loonaris.tech  →  A record  →  NLB DNS name (alias)
```

Client connection strings use this domain:
```text
postgres://sk_live_BASEKEY_rw@db.loonaris.tech:5432/app
```

Inside EKS, the gateway connects to tenant PgBouncer services using
Kubernetes internal DNS:
```text
pooler-rw-svc.project-abc12345.svc.cluster.local:5432
pooler-ro-svc.project-abc12345.svc.cluster.local:5432
```

---

## 7. Tenant Provisioning — Gateway Integration

When the Express backend provisions a new tenant, it must:

### 7.1 Generate API Keys

```typescript
import crypto from 'crypto';

function generateApiKey(mode: 'rw' | 'ro'): { plain: string; hash: string } {
  const baseKey = crypto.randomBytes(32).toString('hex'); // 64 hex chars
  const plain = `sk_live_${baseKey}_${mode}`;
  const hash = crypto.createHash('sha256').update(baseKey).digest('hex');
  return { plain, hash };
}

// For each project:
const rwKey = generateApiKey('rw');
const roKey = generateApiKey('ro');
// Store rwKey.hash and roKey.hash in the api_keys table
// Return rwKey.plain and roKey.plain to the user ONE TIME ONLY
```

### 7.2 Control Plane API Endpoint

The Express backend must expose `GET /api/internal/routes/:keyHash`. This endpoint:

1. Authenticates via `Authorization: Bearer <INTERNAL_GATEWAY_SECRET>`
2. Queries the database joining `ApiKey` → `Project` → `Pooler`
3. Returns the JSON schema the gateway expects

**Express route definition** (to be implemented in `/web-app`):

```typescript
// web-app/backend/src/modules/internal/routes.ts
router.get('/internal/routes/:keyHash', internalAuth, async (req, res) => {
  const { keyHash } = req.params;
  const apiKey = await prisma.apiKey.findUnique({
    where: { keyHash },
    include: { project: { include: { poolers: true } } },
  });

  if (!apiKey || apiKey.revokedAt) return res.status(404).json({ error: 'not found' });
  if (apiKey.project.status !== 'running') return res.status(200).json({ ...route, status: apiKey.project.status });

  const ns = apiKey.project.k8sNamespace;
  res.json({
    tenant_id: ns,
    pgbouncer_rw_host: `pooler-rw-svc.${ns}.svc.cluster.local`,
    pgbouncer_rw_port: 5432,
    pgbouncer_ro_host: `pooler-ro-svc.${ns}.svc.cluster.local`,
    pgbouncer_ro_port: 5432,
    status: 'active',  // Map 'running' → 'active'
  });
});
```

**Critical mapping:** The Prisma schema uses `ProjectStatus.running` but the gateway
requires `status: "active"`. The handler must transform this.

### 7.3 Status Lifecycle Mapping

| Database Status | Gateway `status` | Gateway Behavior |
|---|---|---|
| `provisioning` | `"provisioning"` | Drop connection |
| `running` | `"active"` (must transform) | Allow — tunnel established |
| `stopped` | `"stopped"` | Drop connection |
| `error` | `"error"` | Drop connection |
| `deleting` | `"deleting"` | Drop connection |

---

## 8. Local Development & Testing

### Quick Start

```bash
# Start the full stack (gateway + stub API + PgBouncers + PostgreSQL)
docker compose up --build

# Test RW connection (tenant 1)
psql "host=localhost port=35432 user=sk_live_aaa...64a_rw dbname=app_test1" \
     -c "SELECT * FROM test_data;"

# Test RO connection (tenant 1)
psql "host=localhost port=35432 user=sk_live_aaa...64a_ro dbname=app_test1" \
     -c "SELECT * FROM test_data;"

# Test invalid key (should drop silently)
psql "host=localhost port=35432 user=invalid_user dbname=testdb"

# Tear down
docker compose down --remove-orphans
```

### Test Tenant Keys

| Tenant | Full RW Key | Full RO Key | SHA256 Hash |
|---|---|---|---|
| test1 | `sk_live_aaa...64a_rw` | `sk_live_aaa...64a_ro` | `ffe054fe7ae0cb6dc65c3af9b61d5209f439851db43d0ba5997337df154668eb` |
| test2 | `sk_live_bbb...64b_rw` | `sk_live_bbb...64b_ro` | `a0fab1377f49a759b57f63318262ebe89fabfc990e8e93ceac2984561482b9d4` |
| test3 | `sk_live_ccc...64c_rw` | `sk_live_ccc...64c_ro` | `52b6419d27bd7f547cee3b92f8c17a908b8a49601ecbec161e5030de1dfe9e0a` |

### Run Unit Tests

```bash
go test ./... -v -count=1
```

---

## 9. Architecture Mapped to EKS Node Pools

| EKS Node Group | Taint | Pod Placement | Purpose |
|---|---|---|---|
| System Plane | `dedicated=system:NoSchedule` | db-gateway | Public ingress + routing |
| Tenant Plane | `dedicated=tenant:NoSchedule` | CNPG, PgBouncer RW, PgBouncer RO | Isolated database workloads |

### Cross-Node Routing

```text
db-gateway (system-plane node inside EKS)
    │
    ├──► https://loonaris.tech/api/internal/routes/{hash}  (cache miss → Nginx → ALB → ECS)
    │
    ├──► pooler-rw-svc.project-abc12345.svc.cluster.local:5432  (rw tunnel)
    │
    └──► pooler-ro-svc.project-abc12345.svc.cluster.local:5432  (ro tunnel)
```

The control plane URL resolves to the ALB which routes to the ECS Fargate tasks
running the Express backend. The PgBouncer FQDNs resolve via Kubernetes CoreDNS
to the tenant-plane pods.

---

## 10. Operational Runbook

### Scale Up/Down

```bash
kubectl scale deployment db-gateway -n system-plane --replicas=3
```

The gateway is stateless (cache is in-memory). Multiple replicas behind the NLB
share load. Cache warming happens naturally as connections arrive.

### Rolling Update

```bash
kubectl rollout restart deployment db-gateway -n system-plane
```

The gateway handles SIGTERM with graceful shutdown. Existing connections drain
before the pod terminates (30s timeout + 5s preStop delay).

### Check Logs

```bash
kubectl logs -f deployment/db-gateway -n system-plane --tail=100
```

### Verify Route Resolution

```bash
# Port-forward to gateway
kubectl port-forward svc/db-gateway 5432:5432 -n system-plane

# Test connection
psql "host=localhost user=sk_live_TESTKEY_rw dbname=app"
```

### Emergency Cache Flush

Cache entries expire after 60 seconds. For immediate invalidation, restart:

```bash
kubectl rollout restart deployment db-gateway -n system-plane
```

---

## 11. Security Checklist

| Concern | Mitigation |
|---|---|
| Plaintext keys on network | PostgreSQL startup packets travel inside EKS (VPC-internal). For external TLS, terminate at NLB with ACM certificate. |
| Key hashing | Only SHA256 hash of base key is sent to control plane. Plaintext never leaves the gateway. |
| Bearer token auth | `INTERNAL_GATEWAY_SECRET` is stored in K8s Secret, not in the image. Same value configured on ECS. |
| Invalid key drop | No bytes returned — client sees "server closed connection". |
| DDoS mitigation | Read deadline (30s) on startup packet. Singleflight deduplication prevents cache-stampede. |
| Cross-tenant isolation | Gateway routes to tenant namespaces via K8s DNS. PgBouncer enforces per-tenant database isolation. |
| Secrets rotation | Rotate `INTERNAL_GATEWAY_SECRET` by updating the K8s Secret and ECS task definition, then rolling both. |