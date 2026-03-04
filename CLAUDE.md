# CLAUDE.md — AI Assistant Guide for sangraha

> Last updated: 2026-03-04
> Repository: madhavkobal/sangraha
> Status: Architecture & Planning Phase

This file is the authoritative reference for AI assistants (Claude Code and similar tools) working in this repository. It describes what the project is, how it is architected, how to build and test it, and the conventions all contributors — human and AI — must follow.

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Architecture Blueprint](#2-architecture-blueprint)
3. [Codebase Structure](#3-codebase-structure)
4. [Technology Stack](#4-technology-stack)
5. [Core Features & Implementation Plan](#5-core-features--implementation-plan)
6. [API Design (S3 Compatibility)](#6-api-design-s3-compatibility)
7. [Security Requirements](#7-security-requirements)
8. [CLI Interface](#8-cli-interface)
9. [Web Dashboard](#9-web-dashboard)
10. [Scalability & Storage Backends](#10-scalability--storage-backends)
11. [Development Setup](#11-development-setup)
12. [Testing Strategy](#12-testing-strategy)
13. [Linting & Formatting](#13-linting--formatting)
14. [Build & Release](#14-build--release)
15. [CI/CD](#15-cicd)
16. [Development Roadmap](#16-development-roadmap)
17. [Key Conventions for AI Assistants](#17-key-conventions-for-ai-assistants)
18. [Git Workflow](#18-git-workflow)

---

## 1. Project Overview

**sangraha** (Sanskrit: संग्रह — "collection" or "repository") is a **single-binary, S3-compatible object storage system** built for self-hosted, on-premise, and edge deployments. It follows an **API-first philosophy**: every feature is accessible via the S3 REST API first, then surfaced through the CLI and web dashboard.

### Design Principles

| Principle | Meaning |
|---|---|
| **API First** | The S3-compatible HTTP API is the source of truth. CLI and UI are thin clients over it. |
| **Single Binary** | One compiled binary contains the storage engine, API server, CLI, and web UI (embedded). |
| **Zero External Dependencies at Runtime** | No separate database, message broker, or cache required for single-node deployment. |
| **Progressive Scalability** | Works on a Raspberry Pi for personal use; scales horizontally for team/enterprise use. |
| **Security by Default** | TLS, authentication, and audit logging are on by default, not opt-in. |
| **Operator Friendly** | Simple YAML/TOML config, health endpoints, Prometheus metrics, structured logs. |

### Target Audience

| Audience | Use Case |
|---|---|
| Individual developers | Local S3 mock for development and testing |
| Small teams / startups | Self-hosted file storage without AWS costs |
| Enterprises | Air-gapped or compliance-constrained environments |
| Edge / IoT operators | Lightweight storage on constrained hardware |
| Open-source contributors | Learning ground for distributed storage concepts |

---

## 2. Architecture Blueprint

```
┌─────────────────────────────────────────────────────────────────┐
│                        sangraha binary                          │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │  S3 API      │  │  Admin REST  │  │  Web Dashboard       │  │
│  │  (port 9000) │  │  API         │  │  (embedded SPA,      │  │
│  │              │  │  (port 9001) │  │   served on 9001)    │  │
│  └──────┬───────┘  └──────┬───────┘  └──────────────────────┘  │
│         │                 │                                     │
│  ┌──────▼─────────────────▼────────────────────────────────┐   │
│  │                  Core Router / Middleware                │   │
│  │  (auth, rate-limit, request-id, audit, metrics)         │   │
│  └──────────────────────────┬──────────────────────────────┘   │
│                             │                                   │
│  ┌──────────────────────────▼──────────────────────────────┐   │
│  │                    Storage Engine                        │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │   │
│  │  │  Bucket Mgr │  │  Object Mgr │  │  Version Store  │ │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────┘ │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │   │
│  │  │  IAM / ACL  │  │  Multipart  │  │  Lifecycle Mgr  │ │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────┘ │   │
│  └──────────────────────────┬──────────────────────────────┘   │
│                             │                                   │
│  ┌──────────────────────────▼──────────────────────────────┐   │
│  │                 Storage Backend Interface                │   │
│  │  (pluggable via Go interface)                           │   │
│  │                                                         │   │
│  │  ┌────────────┐  ┌────────────┐  ┌───────────────────┐ │   │
│  │  │  Local FS  │  │  BadgerDB  │  │  (future: S3,     │ │   │
│  │  │  (default) │  │  (embedded)│  │   GCS, Azure)     │ │   │
│  │  └────────────┘  └────────────┘  └───────────────────┘ │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │               Metadata Store                            │   │
│  │  BoltDB/bbolt (embedded KV, single-file, no deps)       │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │  Prometheus  │  │  Structured  │  │  Audit Log           │  │
│  │  Metrics     │  │  Logger      │  │  (append-only file   │  │
│  │  /metrics    │  │  (zerolog)   │  │   or syslog)         │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Request Flow (S3 PutObject Example)

```
Client → TLS termination → Auth middleware (HMAC-SHA256 SigV4)
       → Rate limiter → Request ID injection → Audit log (start)
       → Router → ObjectHandler.Put()
       → Storage Engine: validate bucket, check ACL, compute ETag
       → Backend.Write(data stream) → Metadata.Put(object record)
       → Audit log (complete) → Response with ETag header
```

---

## 3. Codebase Structure

The repository follows **standard Go project layout**. All packages are internal by default; only the public API surface is exported.

```
sangraha/
├── CLAUDE.md                  ← This file
├── README.md
├── LICENSE
├── go.mod
├── go.sum
│
├── cmd/
│   └── sangraha/
│       └── main.go            ← Binary entry point; wires everything together
│
├── internal/
│   ├── api/
│   │   ├── s3/                ← S3-compatible HTTP handlers
│   │   │   ├── router.go      ← gorilla/mux or chi router setup
│   │   │   ├── bucket.go      ← CreateBucket, DeleteBucket, ListBuckets, HeadBucket
│   │   │   ├── object.go      ← PutObject, GetObject, DeleteObject, HeadObject, CopyObject
│   │   │   ├── multipart.go   ← CreateMultipartUpload, UploadPart, CompleteMultipart, AbortMultipart
│   │   │   ├── list.go        ← ListObjectsV2, ListObjectVersions
│   │   │   └── errors.go      ← S3 XML error response builder
│   │   ├── admin/             ← Admin REST API (non-S3)
│   │   │   ├── router.go
│   │   │   ├── users.go       ← User / access key management
│   │   │   ├── buckets.go     ← Admin bucket operations (quota, policy)
│   │   │   ├── metrics.go     ← Prometheus metrics endpoint
│   │   │   └── health.go      ← /healthz, /readyz
│   │   └── middleware/
│   │       ├── auth.go        ← SigV4 + bearer token verification
│   │       ├── ratelimit.go
│   │       ├── requestid.go
│   │       ├── audit.go       ← Audit log emission
│   │       └── tls.go         ← TLS config helper
│   │
│   ├── storage/
│   │   ├── engine.go          ← Core StorageEngine struct; orchestrates all sub-systems
│   │   ├── bucket.go          ← Bucket lifecycle logic
│   │   ├── object.go          ← Object CRUD, ETag computation, content-type detection
│   │   ├── version.go         ← Versioning logic (enabled/suspended/disabled per bucket)
│   │   ├── multipart.go       ← Multipart upload state machine
│   │   ├── lifecycle.go       ← Expiration, transition rules (cron-driven)
│   │   └── replication.go     ← (Phase 3) async object replication
│   │
│   ├── backend/
│   │   ├── interface.go       ← Backend interface definition (CRITICAL — never change without review)
│   │   ├── localfs/           ← Default: local filesystem backend
│   │   │   └── localfs.go
│   │   └── badger/            ← Optional: BadgerDB backend (good for small/embedded deployments)
│   │       └── badger.go
│   │
│   ├── metadata/
│   │   ├── store.go           ← Metadata store interface
│   │   └── bbolt/             ← Default: bbolt (BoltDB) embedded KV
│   │       └── bbolt.go
│   │
│   ├── auth/
│   │   ├── sigv4.go           ← AWS Signature Version 4 implementation
│   │   ├── iam.go             ← IAM policy evaluation engine
│   │   ├── acl.go             ← Bucket/object ACL (canned + custom)
│   │   └── tokens.go          ← Static access key / secret key management
│   │
│   ├── config/
│   │   ├── config.go          ← Config struct (YAML/TOML/env-var parsing via viper)
│   │   └── defaults.go        ← Sensible default values
│   │
│   ├── audit/
│   │   └── audit.go           ← Structured audit event emission
│   │
│   └── web/
│       └── embed.go           ← go:embed directive for web dashboard static assets
│
├── web/                       ← Web dashboard frontend source
│   ├── package.json
│   ├── vite.config.ts
│   ├── src/
│   │   ├── main.ts
│   │   ├── App.tsx
│   │   ├── pages/
│   │   │   ├── Dashboard.tsx
│   │   │   ├── Buckets.tsx
│   │   │   ├── Objects.tsx
│   │   │   ├── Users.tsx
│   │   │   └── Settings.tsx
│   │   ├── components/
│   │   └── api/               ← Admin API client (typed fetch wrappers)
│   └── dist/                  ← Built output — committed or generated at build time
│
├── cli/                       ← CLI command implementations (cobra)
│   ├── root.go
│   ├── server.go              ← `sangraha server start/stop/status`
│   ├── bucket.go              ← `sangraha bucket create/delete/list/policy`
│   ├── object.go              ← `sangraha object put/get/delete/list/cp/mv`
│   ├── user.go                ← `sangraha user create/delete/list/rotate-key`
│   ├── config.go              ← `sangraha config show/set/validate`
│   └── admin.go               ← `sangraha admin export/import/gc`
│
├── pkg/
│   └── s3types/               ← Exported S3 XML type definitions (reusable by other projects)
│
├── scripts/
│   ├── build.sh               ← Cross-platform build script
│   ├── gen-certs.sh           ← Self-signed cert generation for dev
│   └── integration-test.sh    ← Runs integration tests against a live binary
│
├── test/
│   ├── integration/           ← Integration tests (start real server, use AWS SDK)
│   │   ├── bucket_test.go
│   │   ├── object_test.go
│   │   └── multipart_test.go
│   └── fixtures/              ← Test data files
│
├── docs/
│   ├── api/                   ← OpenAPI specs for Admin API
│   ├── architecture/          ← ADR (Architecture Decision Records)
│   └── config-reference.md
│
└── .github/
    └── workflows/
        ├── ci.yml
        └── release.yml
```

---

## 4. Technology Stack

### Language: Go 1.22+

Go is the only justified choice for this project:
- Single static binary compilation — no runtime, no interpreter to ship
- Native HTTP/2, TLS, and concurrency primitives (goroutines, channels)
- Excellent performance for I/O-bound storage workloads
- Strong standard library: `net/http`, `crypto`, `encoding/xml`
- Cross-compilation for Linux/macOS/Windows/ARM without containers

### Key Dependencies

| Package | Purpose | Why |
|---|---|---|
| `github.com/go-chi/chi/v5` | HTTP router | Lightweight, idiomatic, middleware-composable |
| `github.com/spf13/cobra` | CLI framework | Industry standard for Go CLIs |
| `github.com/spf13/viper` | Config management | Multi-source config (file + env + flags) |
| `go.etcd.io/bbolt` | Metadata KV store | Embedded, single-file, ACID, no external dep |
| `github.com/dgraph-io/badger/v4` | Optional object backend | High-perf embedded KV for small deployments |
| `github.com/rs/zerolog` | Structured logging | Zero-allocation JSON logger |
| `github.com/prometheus/client_golang` | Metrics | Standard Prometheus instrumentation |
| `github.com/minio/minio-go/v7` | Integration test client | Official S3 SDK to test against |
| `golang.org/x/crypto` | TLS, bcrypt, HKDF | Extended crypto primitives |
| `github.com/google/uuid` | Request/upload IDs | RFC 4122 UUIDs |

### Frontend (Web Dashboard)

| Tool | Purpose |
|---|---|
| React 18 + TypeScript | UI framework |
| Vite | Build tool (fast HMR, small output) |
| TanStack Query | API data fetching / caching |
| shadcn/ui + Tailwind CSS | Component library |

The built `web/dist/` is embedded into the Go binary via `//go:embed` at compile time. No separate static file server required.

### What is explicitly NOT used

| Avoided | Reason |
|---|---|
| gRPC (for S3 API) | S3 clients expect HTTP/REST/XML; gRPC would break compatibility |
| ORM / SQL database | Adds runtime dependency; bbolt is sufficient for metadata at scale |
| Docker (as a runtime requirement) | Contradicts single-binary goal |
| gorilla/mux | chi is lighter and actively maintained |

---

## 5. Core Features & Implementation Plan

### Phase 1 — Functional Core (MVP)

| Feature | Package | Notes |
|---|---|---|
| Bucket CRUD | `internal/storage/bucket.go` | CreateBucket, DeleteBucket, ListBuckets, HeadBucket |
| Object CRUD | `internal/storage/object.go` | PutObject, GetObject, DeleteObject, HeadObject |
| ETag computation | `internal/storage/object.go` | MD5 of content; multipart uses composite ETag |
| ListObjectsV2 | `internal/api/s3/list.go` | Prefix, delimiter, continuation token pagination |
| Multipart upload | `internal/storage/multipart.go` | 5MB minimum part size (S3 spec) |
| HMAC-SHA256 SigV4 auth | `internal/auth/sigv4.go` | Required for all AWS SDK clients |
| Local filesystem backend | `internal/backend/localfs/` | Objects as files; path = `<data-dir>/<bucket>/<key>` |
| bbolt metadata store | `internal/metadata/bbolt/` | Bucket registry, object index, upload state |
| TLS (auto self-signed) | `internal/api/middleware/tls.go` | Auto-generate if cert not provided |
| Config file | `internal/config/` | YAML; env var overrides |
| Basic CLI | `cli/` | server, bucket, object, user subcommands |
| Health endpoints | `internal/api/admin/health.go` | `/healthz` (liveness), `/readyz` (readiness) |
| Prometheus metrics | `internal/api/admin/metrics.go` | Request count, latency, bytes transferred |
| Structured logging | everywhere | zerolog, JSON output, log level control |

### Phase 2 — Production Hardening

| Feature | Notes |
|---|---|
| Object versioning | Per-bucket enable/disable; version IDs on all mutations |
| Bucket ACLs | Canned ACLs: private, public-read, public-read-write, authenticated-read |
| Bucket policies (JSON) | Subset of AWS IAM policy language targeting S3 actions |
| Object tagging | Key-value tags on objects; filterable in lifecycle rules |
| Lifecycle rules | Expiration by age, transition between storage classes |
| Server-side encryption | AES-256-GCM with per-object or per-bucket keys; key stored in metadata |
| Presigned URLs | Time-limited, signature-authenticated URLs for object GET/PUT |
| CORS configuration | Per-bucket CORS rules for browser clients |
| Rate limiting | Per-IP and per-access-key token bucket |
| Audit log | Append-only structured log; every API call recorded |
| Web dashboard | Embedded React SPA served from admin port |

### Phase 3 — Scale & Enterprise

| Feature | Notes |
|---|---|
| Static website hosting | Serve bucket contents as a static site |
| Object replication | Async push to secondary sangraha instance or upstream S3 |
| Quota management | Per-bucket and per-user storage quotas |
| Event notifications | Webhook on object PUT/DELETE (S3 event notification compatible) |
| Multi-node clustering | Raft-based leader election; shared metadata via etcd or distributed bbolt |
| External auth | LDAP / OIDC / SAML integration for SSO |
| Storage tiering | Hot / warm / cold tiers with configurable backends per tier |

---

## 6. API Design (S3 Compatibility)

### S3 API Endpoint Coverage (Phase 1 target)

```
# Bucket operations
PUT    /{bucket}                        CreateBucket
DELETE /{bucket}                        DeleteBucket
HEAD   /{bucket}                        HeadBucket
GET    /                                ListBuckets
GET    /{bucket}?list-type=2            ListObjectsV2
GET    /{bucket}?versions               ListObjectVersions

# Object operations
PUT    /{bucket}/{key}                  PutObject
GET    /{bucket}/{key}                  GetObject
DELETE /{bucket}/{key}                  DeleteObject
HEAD   /{bucket}/{key}                  HeadObject
PUT    /{bucket}/{key} (x-amz-copy-source) CopyObject

# Multipart
POST   /{bucket}/{key}?uploads          CreateMultipartUpload
PUT    /{bucket}/{key}?partNumber=&uploadId= UploadPart
POST   /{bucket}/{key}?uploadId=        CompleteMultipartUpload
DELETE /{bucket}/{key}?uploadId=        AbortMultipartUpload
GET    /{bucket}/{key}?uploadId=        ListParts
GET    /{bucket}?uploads                ListMultipartUploads

# Batch
POST   /{bucket}?delete                 DeleteObjects
```

### Admin REST API (non-S3, JSON)

```
GET    /admin/v1/health
GET    /admin/v1/ready
GET    /admin/v1/metrics          (Prometheus text format)
GET    /admin/v1/info             (version, uptime, storage stats)

POST   /admin/v1/users
GET    /admin/v1/users
DELETE /admin/v1/users/{username}
POST   /admin/v1/users/{username}/keys/rotate

GET    /admin/v1/buckets          (with quota / policy detail)
PUT    /admin/v1/buckets/{name}/quota
GET    /admin/v1/audit?from=&to=&user=&bucket=
POST   /admin/v1/gc               (trigger garbage collection)
```

### Error Response Format

All S3 errors return AWS-compatible XML:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>The specified bucket does not exist</Message>
  <RequestId>abc123</RequestId>
  <Resource>/my-bucket</Resource>
</Error>
```

---

## 7. Security Requirements

### Authentication

- **AWS Signature Version 4 (SigV4)** for all S3 API calls — mandatory, not optional
- **Bearer token** (HMAC-signed JWT) for admin API and web dashboard
- Access key + secret key pairs stored with bcrypt-hashed secrets
- Root/admin user configured at first boot via `sangraha init`

### Authorization

- **IAM-style bucket policies**: JSON documents evaluated per request
- **Canned ACLs**: `private` (default), `public-read`, `public-read-write`
- **Admin vs. regular user** role distinction for admin API access
- Policy evaluation order: explicit deny > explicit allow > default deny

### Encryption

| Layer | Mechanism |
|---|---|
| Transport | TLS 1.2+ enforced; TLS 1.3 preferred; auto self-signed cert if none provided |
| At-rest (optional) | AES-256-GCM; per-object envelope encryption; key stored encrypted in metadata |
| Secret key storage | bcrypt (cost ≥ 12) for stored secrets |
| Presigned URL signing | HMAC-SHA256 with configurable expiry (max 7 days, matching S3) |

### Audit Logging

Every API request emits a structured audit event:

```json
{
  "time": "2026-03-04T12:00:00Z",
  "request_id": "01HXYZ...",
  "user": "alice",
  "action": "s3:PutObject",
  "bucket": "my-bucket",
  "key": "photos/cat.jpg",
  "source_ip": "192.168.1.5",
  "status": 200,
  "bytes": 204800,
  "duration_ms": 12
}
```

Audit log is append-only. It can be written to a local file or shipped to syslog. Deletion of audit log entries requires explicit operator action with a confirmation flag.

### Security Non-Negotiables for AI Assistants

- **Never** weaken TLS config (e.g., do not add `InsecureSkipVerify: true`)
- **Never** add a "disable auth" flag or bypass auth for any code path
- **Never** log secret keys, passwords, or full request bodies containing sensitive data
- **Never** store access key secrets in plaintext — always bcrypt
- **Never** allow path traversal in object keys (validate and sanitize before filesystem operations)
- **Never** trust user-supplied `Content-Length` alone — stream and verify

---

## 8. CLI Interface

The CLI is built with **cobra** and communicates with a running sangraha server via its Admin API and S3 API.

### Command Structure

```
sangraha
├── init                          # First-time setup: config, certs, root user
├── server
│   ├── start                     # Start the server (foreground or --daemon)
│   ├── stop                      # Graceful shutdown via admin API
│   └── status                    # Show running status, version, storage stats
├── bucket
│   ├── create <name>             # With optional --region, --acl
│   ├── delete <name>             # With --force to bypass non-empty check
│   ├── list                      # Table output; --json for scripting
│   ├── policy get <name>
│   ├── policy set <name> <file>
│   └── versioning <name> enable|disable|suspend
├── object
│   ├── put <bucket> <key> <file> # With --content-type, --metadata
│   ├── get <bucket> <key>        # To stdout or --output <file>
│   ├── delete <bucket> <key>
│   ├── list <bucket>             # With --prefix, --delimiter, --json
│   ├── cp <src> <dst>            # s3://bucket/key URIs supported
│   └── mv <src> <dst>
├── user
│   ├── create <username>         # Generates access key + secret; prints once
│   ├── delete <username>
│   ├── list
│   └── rotate-key <username>
├── config
│   ├── show                      # Current effective config
│   ├── set <key> <value>
│   └── validate                  # Parse and validate config file, report errors
└── admin
    ├── export <output-dir>       # Full data + metadata export
    ├── import <input-dir>
    └── gc                        # Trigger garbage collection of orphaned objects
```

### CLI Conventions

- Output is human-readable table by default; `--json` switches to machine-readable JSON
- `--server` flag or `SANGRAHA_SERVER` env var points to server address
- `--access-key` / `--secret-key` or `SANGRAHA_ACCESS_KEY` / `SANGRAHA_SECRET_KEY`
- Destructive commands (`delete`, `gc`) require `--confirm` flag unless `--force` is set
- Exit codes: 0 = success, 1 = usage error, 2 = server error, 3 = not found

---

## 9. Web Dashboard

The web dashboard is a React SPA embedded in the binary and served on the admin port (`9001` default).

### Pages

| Page | Key Functionality |
|---|---|
| **Dashboard** | Storage usage gauges, request rate graphs (Prometheus data), top buckets by size |
| **Buckets** | List, create, delete buckets; edit versioning, ACL, CORS, lifecycle rules |
| **Objects** | Browse bucket contents; upload (drag-and-drop); download; delete; view metadata/tags |
| **Users** | Create/delete users; rotate access keys; assign policies |
| **Settings** | TLS config, server address, log level, storage backend selection |
| **Audit Log** | Searchable, filterable audit log viewer |

### UX Principles

- No external CDN dependencies — all assets embedded in binary
- Dark mode by default (system-preference aware)
- Responsive for tablet/desktop (not a mobile-first app)
- All destructive actions require a confirmation modal with the resource name typed in
- API errors displayed inline with the raw error code for debuggability

---

## 10. Scalability & Storage Backends

### Backend Interface

All backends implement this interface. **Never change this interface without an ADR.**

```go
// internal/backend/interface.go
type Backend interface {
    // Write streams object data; returns bytes written or error
    Write(ctx context.Context, bucket, key string, r io.Reader, size int64) (int64, error)
    // Read streams object data to w
    Read(ctx context.Context, bucket, key string, w io.Writer) error
    // Delete removes an object
    Delete(ctx context.Context, bucket, key string) error
    // Exists checks if an object exists without reading it
    Exists(ctx context.Context, bucket, key string) (bool, error)
    // Stat returns size and modification time
    Stat(ctx context.Context, bucket, key string) (ObjectInfo, error)
}
```

### Supported Backends

| Backend | Use Case | Max Recommended Size |
|---|---|---|
| `localfs` | Default; objects as files on disk | Limited by disk; tested to 50TB |
| `badger` | Embedded KV; fast small-object workloads | ~100GB before memory pressure |
| `s3proxy` (Phase 3) | Tiering: front sangraha caches upstream S3 | Unlimited (upstream-bound) |

### Single-Node Performance Targets

| Metric | Target |
|---|---|
| Small object PUT (4KB) | ≥ 5,000 req/s |
| Large object PUT (1GB) | saturate disk I/O |
| Concurrent connections | 10,000 (Go net/http default + tuning) |
| Metadata operations | ≥ 50,000 req/s (bbolt read path) |

### Horizontal Scaling (Phase 3)

- Stateless API nodes; shared metadata via etcd or distributed bbolt
- Object storage on shared NFS, Ceph, or object-backed filesystem
- Raft consensus for cluster membership and leader election
- Load balancer (nginx, HAProxy, or cloud LB) in front of API nodes

---

## 11. Development Setup

### Prerequisites

```bash
go 1.22+          # https://go.dev/dl/
node 20+          # for web dashboard development
make              # build automation
openssl           # for dev cert generation
```

### Clone and Bootstrap

```bash
git clone <repo-url> sangraha
cd sangraha

# Install Go dependencies
go mod download

# Install frontend dependencies
cd web && npm install && cd ..

# Generate development TLS certificates
./scripts/gen-certs.sh

# Build everything (binary + embedded web assets)
make build

# Run in development mode (auto-reloads config; no TLS)
./sangraha server start --dev
```

### Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `SANGRAHA_CONFIG` | `$HOME/.sangraha/config.yaml` | Config file path |
| `SANGRAHA_DATA_DIR` | `$HOME/.sangraha/data` | Object storage root |
| `SANGRAHA_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `SANGRAHA_ACCESS_KEY` | — | CLI: access key for auth |
| `SANGRAHA_SECRET_KEY` | — | CLI: secret key for auth |
| `SANGRAHA_SERVER` | `https://localhost:9000` | CLI: server address |
| `SANGRAHA_ADMIN_URL` | `https://localhost:9001` | CLI: admin API address |

### Config File Structure

```yaml
# ~/.sangraha/config.yaml
server:
  s3_address: ":9000"
  admin_address: ":9001"
  tls:
    enabled: true
    cert_file: "/etc/sangraha/tls.crt"
    key_file: "/etc/sangraha/tls.key"
    auto_self_signed: true   # generate if files absent

storage:
  backend: localfs           # localfs | badger
  data_dir: "/var/lib/sangraha/data"

metadata:
  path: "/var/lib/sangraha/meta.db"

auth:
  root_access_key: "root"    # override via SANGRAHA_ROOT_ACCESS_KEY
  # root_secret_key set via env only — never in config file

logging:
  level: info                # debug | info | warn | error
  format: json               # json | text
  audit_log: "/var/log/sangraha/audit.log"

limits:
  max_object_size: "5TB"
  max_bucket_count: 1000
  rate_limit_rps: 1000       # per access key
```

---

## 12. Testing Strategy

### Unit Tests

- Location: `_test.go` files alongside source
- Run: `go test ./...`
- Coverage target: **≥ 80%** for `internal/` packages
- Mock backend and metadata store via interfaces for isolation

### Integration Tests

- Location: `test/integration/`
- Use the official `minio-go` SDK to exercise the real binary
- Run: `./scripts/integration-test.sh` (starts binary, runs tests, tears down)
- Test matrix: localfs backend, badger backend, TLS on, TLS off

### S3 Compatibility Tests

- Use **s3-tests** (Ceph's S3 compatibility test suite) against running binary
- Run in CI against every PR

### Key Test Commands

```bash
# Unit tests
go test ./...

# Unit tests with coverage
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# Unit tests with race detector
go test -race ./...

# Single package
go test ./internal/auth/...

# Integration tests (requires built binary)
make build && ./scripts/integration-test.sh

# Benchmarks
go test -bench=. -benchmem ./internal/storage/...
```

### Test Conventions

- Table-driven tests preferred over repetitive `TestXxx` functions
- Use `t.Cleanup()` not `defer` for resource teardown in tests
- Do not use `time.Sleep` in tests — use channels or `sync.WaitGroup`
- Integration tests use port `19000` / `19001` to avoid conflicts

---

## 13. Linting & Formatting

### Tools

```bash
# Format (mandatory — CI enforces this)
gofmt -w .
# or
goimports -w .

# Lint
golangci-lint run

# Vet
go vet ./...

# Security scan
gosec ./...

# Frontend
cd web && npm run lint    # ESLint + TypeScript checks
cd web && npm run format  # Prettier
```

### golangci-lint Configuration

`.golangci.yml` is the authoritative lint config. Key enabled linters:
- `errcheck` — all errors must be handled
- `gosimple`, `staticcheck` — idiomatic Go enforcement
- `govet` — correctness checks
- `unused` — no dead code
- `gosec` — security anti-patterns
- `gocyclo` — max cyclomatic complexity 15

### Non-Negotiables

- `gofmt` compliance is required — no exceptions
- All exported symbols must have doc comments
- `err` must never be silently ignored (no `_ = someFunc()` without a comment explaining why)

---

## 14. Build & Release

### Makefile Targets

```makefile
make build          # Build binary for current OS/arch into ./bin/sangraha
make build-all      # Cross-compile for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
make web            # Build web dashboard (outputs to web/dist/)
make test           # Run unit tests
make lint           # Run golangci-lint
make clean          # Remove build artifacts
make release        # Tag, build all platforms, generate checksums
```

### Single Binary Build

```bash
# The web dashboard is embedded; build web first
cd web && npm run build && cd ..

# Build with version info injected
go build \
  -ldflags "-X main.version=$(git describe --tags) -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o bin/sangraha \
  ./cmd/sangraha
```

### Release Artifacts

Each release publishes:
- `sangraha-linux-amd64`
- `sangraha-linux-arm64`
- `sangraha-darwin-amd64`
- `sangraha-darwin-arm64`
- `sangraha-windows-amd64.exe`
- `SHA256SUMS`
- Docker image: `ghcr.io/madhavkobal/sangraha:<version>` (wraps the binary; not required)

### Versioning

Semantic versioning: `MAJOR.MINOR.PATCH`
- `MAJOR`: breaking changes to S3 API compatibility or config schema
- `MINOR`: new features, new S3 operations, new CLI commands
- `PATCH`: bug fixes, security patches, performance improvements

---

## 15. CI/CD

### GitHub Actions Workflows

**`.github/workflows/ci.yml`** — runs on every PR and push to `main`:
1. `go vet ./...`
2. `golangci-lint run`
3. `go test -race ./...`
4. Build binary
5. Run integration tests
6. Run S3 compatibility tests

**`.github/workflows/release.yml`** — runs on version tags (`v*`):
1. All CI steps
2. Cross-compile all platforms
3. Build and push Docker image
4. Create GitHub release with artifacts and checksums

### Branch Protection

- `main` requires passing CI and 1 code review
- Direct pushes to `main` are blocked
- All Claude Code branches follow `claude/<slug>-<session-id>` convention

---

## 16. Development Roadmap

### Milestone 1 — Functional MVP (Target: Phase 1 complete)
- [ ] Project scaffolding: go.mod, directory layout, Makefile
- [ ] Config loading (viper: YAML + env vars)
- [ ] bbolt metadata store
- [ ] Local filesystem backend
- [ ] S3 SigV4 authentication
- [ ] Bucket CRUD (CreateBucket, DeleteBucket, ListBuckets, HeadBucket)
- [ ] Object CRUD (PutObject, GetObject, DeleteObject, HeadObject, CopyObject)
- [ ] ListObjectsV2 with pagination
- [ ] Multipart upload
- [ ] TLS (auto self-signed)
- [ ] Health + readiness endpoints
- [ ] Prometheus metrics
- [ ] Structured logging (zerolog)
- [ ] Basic cobra CLI (server, bucket, object, user)
- [ ] Unit tests ≥ 80% coverage
- [ ] Integration tests with minio-go
- [ ] README and docs

### Milestone 2 — Production Hardening
- [ ] Object versioning
- [ ] Bucket ACLs + policies
- [ ] Object tagging
- [ ] Server-side encryption (AES-256-GCM)
- [ ] Presigned URLs
- [ ] CORS
- [ ] Rate limiting
- [ ] Audit logging
- [ ] Web dashboard (React SPA embedded)
- [ ] Lifecycle rules
- [ ] `sangraha init` wizard

### Milestone 3 — Scale & Enterprise
- [ ] Object replication
- [ ] Webhook event notifications
- [ ] Quota management
- [ ] Static website hosting
- [ ] OIDC/LDAP integration
- [ ] Multi-node clustering (Raft)
- [ ] Storage tiering

---

## 17. Key Conventions for AI Assistants

These rules apply to all code changes. Read them before touching any file.

### Go Code Style

- Follow `gofmt` / `goimports` formatting — no exceptions
- Exported types, functions, and methods require doc comments
- Use `context.Context` as the first parameter on any function that does I/O
- Return errors; never panic in library code (only in `main()` for fatal init errors)
- Use `fmt.Errorf("...: %w", err)` for error wrapping — never discard error context
- Prefer table-driven tests; keep test helpers in `_test.go` files
- Do not use `init()` functions outside of `cmd/`
- Interfaces belong in the package that **uses** them, not the package that implements them (except `internal/backend/interface.go` which is the intentional exception)

### S3 Compatibility Rules

- The S3 API **must** return AWS-compatible XML error responses — never return JSON from S3 endpoints
- ETag values **must** be quoted strings: `"d41d8cd98f00b204e9800998ecf8427e"`
- Bucket names **must** be validated against S3 rules (3-63 chars, lowercase, no consecutive dots)
- Object keys are arbitrary UTF-8 strings up to 1024 bytes — do not impose additional restrictions
- HTTP status codes must match S3 spec exactly (e.g., 200 for PutObject, 204 for DeleteObject)

### Security Rules

- **Never** weaken TLS settings (no `InsecureSkipVerify`, no TLS 1.0/1.1)
- **Never** add code paths that skip authentication
- **Never** log secret keys, passwords, or sensitive header values
- **Never** trust `../` sequences in object keys — validate and reject
- **Never** use `math/rand` for security-sensitive randomness — use `crypto/rand`
- All user input at API boundaries must be validated before use

### Change Discipline

- Make the minimum change necessary to accomplish the task
- Do not refactor surrounding code unless it is the task
- Do not add comments to code you did not change
- Do not add error handling for scenarios the type system already prevents
- When adding a new S3 operation, add a corresponding integration test

### Naming Conventions

| Thing | Convention | Example |
|---|---|---|
| Go packages | lowercase, single word | `storage`, `auth`, `backend` |
| Go interfaces | noun or noun phrase | `Backend`, `MetadataStore` |
| Go structs | PascalCase noun | `StorageEngine`, `BucketRecord` |
| Go errors | `Err` prefix or sentinel | `ErrBucketNotFound`, `ErrKeyTooLong` |
| Config keys | snake_case in YAML | `data_dir`, `max_object_size` |
| CLI commands | lowercase hyphenated | `rotate-key`, `list-type` |
| API routes | lowercase hyphenated path segments | `/admin/v1/rotate-key` |

---

## 18. Git Workflow

### Branch Naming

Claude Code branches **must** follow:

```
claude/<slug>-<session-id>
```

- Must start with `claude/`
- Must end with the session ID from the current Claude Code session
- Pushing to any other pattern results in a **403 HTTP error**
- Never push to `main` or `master` without explicit user permission

### Commit Messages

Format: `<type>: <short imperative description>`

```
feat: add presigned URL generation for GetObject
fix: correct ETag quoting in multipart complete response
docs: update config reference for TLS settings
test: add integration test for ListObjectsV2 pagination
refactor: extract SigV4 date parsing to helper function
chore: bump golangci-lint to 1.57
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `perf`, `security`

### Push Procedure

```bash
# First push on new branch
git push -u origin claude/<slug>-<session-id>

# On network failure: retry with exponential backoff
# Wait 2s → retry → wait 4s → retry → wait 8s → retry → wait 16s → retry
```

### Fetch / Pull

```bash
git fetch origin <branch-name>
git pull origin <branch-name>
```

---

*AI assistants: treat this file as the ground truth. When in doubt about a convention, check here first. When you establish a new convention through implementation, add it here.*
