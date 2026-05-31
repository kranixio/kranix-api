# kranix-api

> REST / gRPC interface — the unified entry point for all Kranix clients.

`kranix-api` is the external-facing API layer of the Kranix platform. It exposes a versioned REST API and a gRPC interface, handles authentication, validates all incoming requests, and delegates work to `kranix-core`. It is the single front door through which `kranix-cli`, `kranix-mcp`, and any third-party tooling communicate with the platform.

---

## What it does

- Exposes a versioned REST API (`/api/v1/...`, `/api/v2/...`) and a gRPC service
- Handles authentication (API keys, JWT, OIDC)
- Validates and sanitizes all incoming requests
- Validates **cron schedules** (standard 5-field expression, optional IANA **`timeZone`**, **`allow`/`forbid`/`replace`** concurrency policy), **GPU** specs, **cross-namespace traffic** namespace names, and **`scheduling.workloadPriority`** (**`critical`** | **`high`** | **`normal`** | **`low`**) via `internal/validation` (spot / preemption flags are passed through to core and runtime)
- Translates HTTP/gRPC requests into `kranix-core` operations
- Streams logs and events back to callers over SSE / gRPC streams
- Exposes **workload rollback** (revision history + revert) and **cost estimation** (pre-deploy and deployed)
- Emits audit logs for every mutating action
- Enforces rate limiting and per-namespace resource quotas
- Supports API versioning with deprecation warnings

---

## Architecture position

```
kranix-cli  ──┐
             ├──►  kranix-api  ──►  kranix-core
kranix-mcp  ──┘
```

`kranix-api` is intentionally thin — it contains no business logic. If you find yourself adding scheduling or policy decisions here, they belong in `kranix-core` instead.

---

## API reference

### Base URL

```
http://localhost:8080/api/v1
```

### Workloads

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/workloads` | Deploy a workload |
| `GET` | `/workloads` | List workloads (`?all_namespaces=true` for cross-namespace) |
| `GET` | `/workloads/:id` | Get a single workload |
| `PATCH` | `/workloads/:id` | Update workload spec |
| `DELETE` | `/workloads/:id` | Remove a workload |
| `POST` | `/workloads/:id/restart` | Restart a workload |
| `GET` | `/workloads/:id/revisions` | List rollback revision history |
| `POST` | `/workloads/:id/rollback` | Revert workload to a previous revision |

### Cost estimation

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/cost/estimate` | Estimate cost for a proposed deployment spec (pre-deploy) |
| `GET` | `/workloads/:id/cost` | Cost breakdown for a deployed workload (`?namespace=&duration=`) |
| `GET` | `/cost/summary` | Namespace cost rollup (`?namespace=&duration=`) |

Used by **kranix-mcp** tools `estimate_deployment_cost`, and by **kranix-cli** `kranix cost`.

### Pods / containers

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/workloads/:id/pods` | List pods for a workload |
| `GET` | `/pods/:id/logs` | Stream pod logs (SSE) |
| `GET` | `/pods/:id/exec` | Exec into a pod (WebSocket) |

### Namespaces

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/namespaces` | Create a namespace |
| `GET` | `/namespaces` | List namespaces |
| `DELETE` | `/namespaces/:id` | Delete a namespace |

### Analysis

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/workloads/:id/analyze` | AI-powered failure analysis with remediation suggestions |
| `POST` | `/manifests/generate` | Generate K8s/KranixApp manifests from intent (legacy response: `{manifest}`) |
| `POST` | `/templates/kranixapp` | Generate full KranixApp template with parsed metadata |
| `POST` | `/ai/ask` | AI assistant query with suggested action |

### Cluster health & MCP suggestions

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/cluster/health` | Cluster-wide health summary (status, pod counts, degraded workloads) |
| `GET` | `/nodes/health` | Per-node and per-backend health scores (0–100) |
| `POST` | `/nodes/{name}/drain` | Cordon node and evict workloads before maintenance |
| `GET` | `/cluster/suggestions` | Context-aware next-action recommendations for MCP agents (`?namespace=&workload=`) |

Used by **kranix-mcp** tools `get_cluster_health`, `suggest_actions`, and auto-append suggestion hints.

### Rate limiting & Quotas

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/quota` | Set namespace quota |
| `GET` | `/quota/{namespace}` | Get namespace quota |
| `GET` | `/quota/{namespace}/usage` | Get quota usage |
| `GET` | `/quota` | List all quotas |
| `DELETE` | `/quota/{namespace}` | Delete namespace quota |

### SSE Streaming

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/sse` | SSE connection for live events |
| `GET` | `/api/sse/stats` | SSE connection statistics |
| `POST` | `/api/sse/broadcast` | Broadcast events (testing) |

### Approval gate

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/approvals` | Create a pending approval for a destructive MCP action |
| `GET` | `/api/v1/approvals` | List pending approvals (optional `agent_id` filter) |
| `GET` | `/api/v1/approvals/{id}` | Get approval status |
| `POST` | `/api/v1/approvals/{id}/resolve` | Approve or deny a pending gate |

Mutating workload handlers (deploy, restart, delete, rollback) broadcast events on the SSE stream when `sse.enabled: true`.

### API Versioning

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/versions` | List all API versions |
| `GET` | `/api/versions/{version}` | Get version information |

---

## Authentication

All requests require an `Authorization` header:

```
Authorization: Bearer <token>
```

Supported token types:

| Type | Use case |
|---|---|
| API key (`krane_...`) | CI/CD, service accounts, MCP agents |
| JWT | Human users via `kranix-cli` |
| OIDC | SSO / enterprise identity providers |

Tokens are issued by `kranix-api` itself or via your OIDC provider. Configure in `config/auth.yaml`.

### MCP agent identity headers

When called by **kranix-mcp**, requests may include agent identity headers for audit and impersonation tracking:

| Header | Description |
|--------|-------------|
| `X-Agent-Id` | Authenticated MCP agent identifier (e.g. `claude-desktop`) |
| `X-Actor` | Actor recorded in audit logs; defaults to `X-Agent-Id` or `api` |

Audit entries store `agent_id` in `Details` when `X-Agent-Id` is present. This aligns MCP-side impersonation guards with the platform audit trail.

### Dry-run (`?dryRun=true`)

Append `?dryRun=true` (or `?dry_run=true`, or header `X-Dry-Run: true`) to any **mutating** request. The API returns a preview JSON payload with `dryRun: true` and does **not** apply changes (no core calls, no audit side effects beyond optional preview logging).

### Cross-namespace workload list

`GET /api/v1/workloads?all_namespaces=true` lists workloads across every namespace in one request (also accepts `cross_namespace=true` or `namespace=*`). Other filters (`phase`, `image`, `team`, etc.) still apply.

### API key IP allowlist

When creating an API key (`POST /api/v1/apikeys`), set `allowedIps` to an array of IPs or CIDRs (e.g. `["203.0.113.10", "10.0.0.0/8"]`). Requests using that key from any other client IP receive `403 Forbidden`. An empty list means no IP restriction.

---

## Project structure

```
kranix-api/
├── cmd/
│   └── api/              # Entry point
├── internal/
│   ├── handlers/         # HTTP handlers (workloads, rollback, cost, cluster, …)
│   ├── suggestions/      # Cluster health and context-aware suggestion engine
│   ├── grpc/             # gRPC service implementations
│   ├── middleware/        # Auth, logging, rate limiting, CORS
│   ├── validation/        # Request validation (schema + business rules)
│   └── stream/           # SSE and WebSocket streaming helpers
├── proto/                # Protobuf definitions
├── config/               # Default config files
├── openapi/              # OpenAPI 3.0 spec (auto-generated)
└── tests/
    ├── unit/
    └── e2e/
```

---

## Getting started

### Prerequisites

- Go 1.22+
- `kranix-core` running (or in-process for local dev)
- Optional: `buf` CLI for protobuf generation

### Run locally

```bash
git clone https://github.com/kranix-io/kranix-api
cd kranix-api
go mod download

# Start with a local kranix-core
go run ./cmd/api --config ./config/local.yaml
```

### Generate protobuf

```bash
buf generate
```

### Run tests

```bash

ratelimit:
  enabled: true
  requests_per_second: 100
  burst_size: 10

sse:
  enabled: true
  max_connections: 1000

apiversion:
  enabled: true
  default_version: "1"
  header_name: "X-API-Version"
  query_param: "version"
go test ./...
go test ./tests/e2e/... -tags e2e   # requires running kranix-core
```

---

## Configuration

```yaml
api:
  port: 8080
  grpc_port: 9090
  read_timeout: 30s
  write_timeout: 30s

auth:
  mode: jwt                        # jwt | apikey | oidc
  jwt_secret: ""                   # set via KRANE_JWT_SECRET env var
  oidc_issuer: ""

core:
  address: "kranix-core:50051"      # gRPC address of kranix-core

logging:
  level: info
  format: json

audit:
  enabled: true
  sink: stdout                     # stdout | file | kafka
```

---

## Pagination (cursor-based)

Large workload and changelog lists support **cursor pagination**:

| Parameter | Description |
|-----------|-------------|
| `limit` | Page size (default `50`, max `500`) |
| `cursor` | Opaque token from previous `page_info.next_cursor` |

```http
GET /api/v1/workloads?namespace=prod&limit=50
GET /api/v1/workloads?cursor=<token>
```

Response includes `page_info` with `has_more`, `next_cursor`, and `total_count`. A `Link: rel="next"` header is set when more pages exist.

## Changelog breaking-change notifications

Subscribe to **email** and/or **webhook** alerts when a **breaking** API release is published:

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/changelog/subscriptions` | Register `webhookUrl` and/or `email` |
| `GET` | `/api/v1/changelog/subscriptions` | List subscriptions |
| `DELETE` | `/api/v1/changelog/subscriptions/{id}` | Remove subscription |
| `POST` | `/api/v1/changelog/releases` | Publish version + entries; notifies on breaking changes |

Registered **webhooks** with event `changelog.breaking` also receive payloads via the existing webhook system.

Configure SMTP under `changelog_notifications.email` in config.

## Workload diff

`GET /api/v1/workloads/{id}/diff` returns **desired spec vs live status** (phase, replicas, resources, image).  
`POST /api/v1/workloads/{id}/diff` accepts a proposed `WorkloadSpec` body and diffs it against live state (used by the CLI).

## Search and filter

`GET /api/v1/workloads` supports query parameters:

| Parameter | Description |
|-----------|-------------|
| `namespace` | Filter by namespace |
| `phase` / `status` | Filter by lifecycle phase (`Running`, `Pending`, …) |
| `image` | Substring match on container image |
| `team`, `environment`, `cost_center` | Match structured tags |
| `label`, `label_value` | Match arbitrary workload labels |

## Resource quota API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/quotas` | List namespace quotas |
| `GET` | `/api/v1/quotas/{namespace}` | Get limits for a namespace |
| `PUT` | `/api/v1/quotas/{namespace}` | Set or update limits (`HardResourceQuota` body) |
| `DELETE` | `/api/v1/quotas/{namespace}` | Remove limits |
| `GET` | `/api/v1/quotas/{namespace}/usage` | View aggregate usage vs limits |

Proxies to **kranix-core** when `core.http_base_url` is configured.

## Bulk operations

`POST /api/v1/workloads/bulk` runs **deploy**, **restart**, or **delete** for many workloads in one request. Set `continueOnError` to keep processing after individual failures. When `core.http_base_url` points at kranix-core, the API forwards the batch to core.

```json
{
  "operation": "restart",
  "continueOnError": true,
  "workloads": [{ "id": "app-a" }, { "id": "app-b" }]
}
```

## Audit trail

- `GET /api/v1/audit` — filter API audit entries (`resource_type`, `resource_id`, `action`, `actor`, `since`, `limit`)
- `GET /api/v1/audit/{entryId}` — single API audit record
- `GET /api/v1/audit/resources/{type}/{id}` — merges API audit entries with **kranix-core** domain events (event sourcing) for full resource history

Enable with `audit.enabled` in config; entries are written for deploy, restart, delete, and bulk operations. MCP agents are recorded via `X-Agent-Id` / `X-Actor` headers.

## Cluster health & MCP suggestions

Endpoints consumed by **kranix-mcp** for context-aware agent guidance:

### `GET /api/v1/cluster/health`

Returns cluster-wide health derived from workload listings when kranix-core is connected:

```json
{
  "status": "healthy",
  "nodesReady": 1,
  "nodesTotal": 1,
  "podsRunning": 12,
  "podsTotal": 14,
  "degradedWorkloads": 0,
  "lastChecked": "2026-05-23T10:00:00Z"
}
```

Status values: `healthy`, `degraded`, `critical`, `unknown`.

### `GET /api/v1/cluster/suggestions`

Returns recommended next MCP tool actions based on cluster state and optional context:

```http
GET /api/v1/cluster/suggestions?namespace=prod&workload=api
```

```json
{
  "clusterStatus": "degraded",
  "context": { "namespace": "prod", "workload": "api" },
  "suggestions": [
    {
      "tool": "analyze_workload",
      "reason": "Run failure analysis on the target workload",
      "priority": "high",
      "inputs": { "name": "api", "namespace": "prod" },
      "confidence": 0.85
    }
  ],
  "generatedAt": "2026-05-23T10:00:00Z"
}
```

### `GET /api/v1/workloads/:id/analyze`

Returns `types.AnalysisResult` including **`suggestions`** (remediation hints) alongside `issues` and `probableFix`. Shared types are defined in `kranix-packages/types`.

## Workload rollback

Endpoints for revision history and instant rollback (proxied to **kranix-core** when configured):

### `GET /api/v1/workloads/{id}/revisions`

Returns stored spec snapshots (newest first):

```json
{
  "workloadId": "api",
  "namespace": "production",
  "count": 3,
  "revisions": [
    {
      "id": "a1b2c3d4",
      "recordedAt": "2026-05-23T10:00:00Z",
      "spec": { "image": "api:v1", "replicas": 2 },
      "changeReason": "update"
    }
  ]
}
```

### `POST /api/v1/workloads/{id}/rollback`

Reverts to a specific revision. Omit `revisionId` to roll back to the immediately previous version:

```json
{ "revisionId": "a1b2c3d4" }
```

```json
{
  "workloadId": "api",
  "namespace": "production",
  "revisionId": "a1b2c3d4",
  "status": "rolled_back",
  "restoredImage": "api:v1"
}
```

Audit action: `workload.rollback`. Used by **kranix-mcp** tools `rollback_workload` and `list_workload_revisions`.

## Cost estimation

Pre-deploy and deployed workload cost endpoints using the shared `kranix-packages/cost` estimator:

### `POST /api/v1/cost/estimate`

Estimate cost before deploying:

```json
{
  "name": "api",
  "namespace": "staging",
  "duration": "30d",
  "spec": {
    "image": "nginx:latest",
    "replicas": 3,
    "resources": { "cpuRequest": "100m", "cpuLimit": "500m" }
  }
}
```

```json
{
  "workloadName": "api",
  "namespace": "staging",
  "totalCost": 42.50,
  "computeCost": 36.12,
  "monthlyCostPerReplica": 14.17,
  "rightsizing": {
    "recommendedCpuRequest": "100m",
    "recommendedCpuLimit": "250m",
    "reason": "low observed utilization vs CPU limit — candidate for rightsizing"
  }
}
```

### `GET /api/v1/workloads/{id}/cost`

Cost for an existing workload. When core is connected, uses live spec; otherwise falls back to the shared estimator.

### `GET /api/v1/cost/summary`

Namespace-level rollup with top workloads by cost.

## Connectivity

| Repo | Relationship |
|---|---|
| `kranix-cli` | Calls this API over HTTP |
| `kranix-mcp` | Calls this API over HTTP on behalf of AI agents; rollback, cost estimate, cluster health/suggestions, agent identity headers |
| `kranix-core` | This API delegates all business logic to core (including rollback via rollout history) |
| `kranix-packages` | Imports shared types (`types/mcp.go`, `types/cost.go`, `types/rollback.go`), cost estimator, auth |

---

## Rate limiting & Quotas

The API enforces rate limiting per client (based on API key or IP address) and per-namespace resource quotas.

### Rate limiting

- **Token bucket algorithm** with configurable requests per second and burst size
- Clients identified by API key (`X-API-Key` header) or IP address
- Rate limit headers returned: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After`

### Namespace quotas

- Set resource limits per namespace (max workloads, CPU, memory, storage)
- Quota usage tracked in real-time
- Quota enforcement prevents resource exhaustion

**Note:** Aggregate **hard limits** keyed by Kubernetes namespace or by team (`kranix.io/team`) are enforced in **`kranix-core`** when `resource_quota.hard_limits` is configured there. This API validates workload shape (including **cron**) and forwards intent to core; it does not re-implement aggregate quota sums.

---

## GraphQL (workload cron types)

The embedded schema in `internal/graphql/graphql.go` includes **`CronScheduleSpec`**, **`CronScheduleStatus`**, and matching **input** types on **`WorkloadSpec`** / **`WorkloadSpecInput`** so typed clients can submit and read cron fields alongside the REST JSON model in `kranix-packages/types`.

---

## SSE Streaming

The API provides Server-Sent Events (SSE) for real-time event streaming:

- **Connection endpoint:** `GET /api/sse?client_id=...&namespace=...`
- **Event types:** `workload.changed`, `workload.created`, `workload.deleted`
- **Filtering:** Subscribe to specific namespaces or all namespaces
- **Automatic reconnection:** Clients can reconnect with retry intervals
- **MCP integration:** kranix-mcp exposes `subscribe_cluster_events` which polls this endpoint for agent push notifications
- **Broadcasts:** Deploy, restart, delete, and rollback handlers emit events when SSE is enabled in config

---

## Approval gate

Human-in-the-loop confirmation for destructive MCP actions:

```yaml
approval:
  enabled: true
  default_ttl: 10m
```

| Endpoint | Purpose |
|----------|---------|
| `POST /api/v1/approvals` | Agent requests approval before delete/rollback/runbook execution |
| `GET /api/v1/approvals/{id}` | Poll approval status |
| `POST /api/v1/approvals/{id}/resolve` | Operator approves or denies |

Resolved approvals are validated by kranix-mcp when the agent retries the target tool with `approval_id`.

---

## KranixApp template generation

Generate full `kranix.io/v1alpha1` KranixApp manifests from natural language or structured input:

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/manifests/generate` | Legacy endpoint — returns `{ "manifest": "<yaml>" }` |
| `POST` | `/api/v1/templates/kranixapp` | Full response with parsed fields, confidence, and YAML |

**Request body** (`KranixAppTemplateRequest`):

```json
{
  "description": "deploy api-server to staging with 2 replicas and spot instances",
  "name": "api-server",
  "namespace": "staging",
  "profile": "spot",
  "features": ["auto-heal"]
}
```

Generation logic lives in `kranix-packages/template` and is shared across kranix-api and kranix-mcp.

---

## Runtime node health and draining

Proxied to kranix-core (and kranix-runtime when `NodeOperations` is wired):

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/nodes/health` | Node and backend health scores 0–100 |
| `POST` | `/api/v1/nodes/{name}/drain` | Cordon node, apply drain taint, evict pods |

**Drain request body:**

```json
{
  "gracePeriodSeconds": 30,
  "ignoreDaemonSets": true,
  "reason": "maintenance"
}
```

**Multi-arch:** set `scheduling.architecture` to `amd64` or `arm64` on workload specs; runtime applies `kubernetes.io/arch` node selection.

---

## Runtime checkpoint, volumes, bandwidth, and plugins

Proxied to kranix-core (and kranix-runtime when extended ops are wired):

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/workloads/{id}/checkpoint` | Pause container (Docker) or scale Deployment to 0 (Kubernetes) |
| `POST` | `/api/v1/workloads/{id}/restore` | Resume from checkpoint |
| `GET` | `/api/v1/workloads/{id}/checkpoints` | List checkpoint metadata for a workload |
| `GET` | `/api/v1/runtime/plugins` | List registered runtime backend plugins |

**Restore request body:**

```json
{
  "checkpointId": "ckpt-123",
  "namespace": "default"
}
```

**Volume and bandwidth fields** on workload create/update (`WorkloadSpec`):

```json
{
  "volumes": [
    {
      "name": "data",
      "size": "10Gi",
      "mountPath": "/data",
      "autoCleanup": true
    }
  ],
  "networkBandwidth": {
    "enabled": true,
    "egressLimit": "10Mbit",
    "ingressLimit": "5Mbit"
  }
}
```

---

## API Versioning

The API supports multiple versions running side by side:

### Version selection

- **Header:** `X-API-Version: 1` or `X-API-Version: 2`
- **Query param:** `?version=1` or `?version=2`
- **URL path:** `/api/v1/...` or `/api/v2/...`

### Version status

- **v1:** Stable, production-ready
- **v2:** Beta, new features

### Deprecation

Deprecated versions return warning headers:
- `X-API-Deprecated: true`
- `X-API-Sunset-Date: YYYY-MM-DD`
- `Warning: 299 - "API version is deprecated"`

---

## OpenAPI spec

The OpenAPI 3.0 spec is auto-generated on build and available at:

- **Local:** `http://localhost:8080/openapi.json`
- **Docs UI:** `http://localhost:8080/docs`

---

## Local testing without kranix-core

For unit tests in **kranix-cli**, **kranix-mcp**, **kranix-web**, or third-party clients, run the in-memory mock from [kranix-packages](https://github.com/kranix-io/kranix-packages):

```bash
cd ../kranix-packages
go run ./cmd/kranix-mock-api -addr :18080 -skip-auth=true
```

Point **`kranix-mock-api`** at the same address for workloads, namespaces, incident runbooks, latency analytics, and **mock cost / rightsizing** — see [kranix-examples](https://github.com/kranix-io/kranix-examples) (`observability/ai-oncall-pagerduty`, `reference-architectures/ml-inference-platform`, `reference-architectures/multi-cloud-failover`, `platform-engineering/self-service-deploy`, `platform-engineering/backstage-kranix-plugin`, `ai-agents/cost-optimization-agent`).

---

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md). Every new endpoint requires: handler, validation, unit test, and an OpenAPI spec entry. No business logic in handlers — delegate to core.

## License

Apache 2.0 — see [LICENSE](./LICENSE).
