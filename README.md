# kranix-api

> REST / gRPC interface — the unified entry point for all Kranix clients.

`kranix-api` is the external-facing API layer of the Kranix platform. It exposes a versioned REST API and a gRPC interface, handles authentication, validates all incoming requests, and delegates work to `kranix-core`. It is the single front door through which `kranix-cli`, `kranix-mcp`, and any third-party tooling communicate with the platform.

---

## What it does

- Exposes a versioned REST API (`/api/v1/...`, `/api/v2/...`) and a gRPC service
- Handles authentication (API keys, JWT, OIDC)
- Validates and sanitizes all incoming requests
- Translates HTTP/gRPC requests into `kranix-core` operations
- Streams logs and events back to callers over SSE / gRPC streams
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
| `GET` | `/workloads` | List all workloads |
| `GET` | `/workloads/:id` | Get a single workload |
| `PATCH` | `/workloads/:id` | Update workload spec |
| `DELETE` | `/workloads/:id` | Remove a workload |
| `POST` | `/workloads/:id/restart` | Restart a workload |

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
| `GET` | `/workloads/:id/analyze` | AI-powered failure analysis |
| `POST` | `/manifests/generate` | Generate K8s manifests from intent |

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

### API Versioning

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/versions` | List all API versions |
| `GET` | `/api/versions/{version}` | Get version information |

---

## Authentication

All requests require an `Authorization` header:, API versioning
│   ├── ratelimit/        # Rate limiting and quota enforcement
│   ├── sse/              # SSE streaming service
│   ├── apiversion/       # API versioning manager

```
Authorization: Bearer <token>
```

Supported token types:

| Type | Use case |
|---|---|
| API key (`krane_...`) | CI/CD, service accounts |
| JWT | Human users via `kranix-cli` |
| OIDC | SSO / enterprise identity providers |

Tokens are issued by `kranix-api` itself or via your OIDC provider. Configure in `config/auth.yaml`.

---

## Project structure

```
kranix-api/
├── cmd/
│   └── api/              # Entry point
├── internal/
│   ├── handlers/         # HTTP handler functions (one file per resource)
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

## Connectivity

| Repo | Relationship |
|---|---|
| `kranix-cli` | Calls this API over HTTP |
| `kranix-mcp` | Calls this API over HTTP on behalf of AI agents |
| `kranix-core` | This API delegates all business logic to core |
| `kranix-packages` | Imports shared types, errors, and auth primitives |

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

---

## SSE Streaming

The API provides Server-Sent Events (SSE) for real-time event streaming:

- **Connection endpoint:** `GET /api/sse?client_id=...&namespace=...`
- **Event types:** `workload.changed`, `workload.created`, `workload.deleted`
- **Filtering:** Subscribe to specific namespaces or all namespaces
- **Automatic reconnection:** Clients can reconnect with retry intervals

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

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md). Every new endpoint requires: handler, validation, unit test, and an OpenAPI spec entry. No business logic in handlers — delegate to core.

## License

Apache 2.0 — see [LICENSE](./LICENSE).
