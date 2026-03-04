# sangraha — Comprehensive Phase-wise Development Plan

> Version: 1.0
> Last updated: 2026-03-04
> Repository: madhavkobal/sangraha
> Status: Planning

This document is the authoritative development roadmap for sangraha. It breaks the project into three major phases, each subdivided into sprints with explicit tasks, acceptance criteria, dependencies, risks, and deliverables. Read CLAUDE.md first for architecture context.

---

## Table of Contents

1. [Plan Overview](#1-plan-overview)
2. [Phase 0 — Project Scaffolding](#2-phase-0--project-scaffolding)
3. [Phase 1 — Functional MVP](#3-phase-1--functional-mvp)
   - [Sprint 1.1 — Foundation](#sprint-11--foundation)
   - [Sprint 1.2 — Storage Core](#sprint-12--storage-core)
   - [Sprint 1.3 — S3 API Layer](#sprint-13--s3-api-layer)
   - [Sprint 1.4 — Auth & Security Baseline](#sprint-14--auth--security-baseline)
   - [Sprint 1.5 — CLI & Observability](#sprint-15--cli--observability)
   - [Sprint 1.6 — MVP Hardening & Release](#sprint-16--mvp-hardening--release)
4. [Phase 2 — Production Hardening](#4-phase-2--production-hardening)
   - [Sprint 2.1 — Versioning & ACLs](#sprint-21--versioning--acls)
   - [Sprint 2.2 — Encryption & Presigned URLs](#sprint-22--encryption--presigned-urls)
   - [Sprint 2.3 — Lifecycle, Tagging & CORS](#sprint-23--lifecycle-tagging--cors)
   - [Sprint 2.4 — Rate Limiting & Audit Log](#sprint-24--rate-limiting--audit-log)
   - [Sprint 2.5 — Web Dashboard](#sprint-25--web-dashboard)
   - [Sprint 2.6 — Production Release](#sprint-26--production-release)
5. [Phase 3 — Scale & Enterprise](#5-phase-3--scale--enterprise)
   - [Sprint 3.1 — Quotas & Event Notifications](#sprint-31--quotas--event-notifications)
   - [Sprint 3.2 — Static Website Hosting](#sprint-32--static-website-hosting)
   - [Sprint 3.3 — Object Replication](#sprint-33--object-replication)
   - [Sprint 3.4 — External Authentication (OIDC/LDAP)](#sprint-34--external-authentication-oidcldap)
   - [Sprint 3.5 — Storage Tiering](#sprint-35--storage-tiering)
   - [Sprint 3.6 — Multi-node Clustering](#sprint-36--multi-node-clustering)
   - [Sprint 3.7 — Enterprise Release](#sprint-37--enterprise-release)
6. [Cross-Cutting Concerns](#6-cross-cutting-concerns)
7. [Dependency Graph](#7-dependency-graph)
8. [Risk Register](#8-risk-register)
9. [Definition of Done](#9-definition-of-done)
10. [Versioning Strategy](#10-versioning-strategy)

---

## 1. Plan Overview

```
Phase 0   Phase 1 (MVP)                        Phase 2 (Production)         Phase 3 (Enterprise)
─────────┬──────────────────────────────────────┬────────────────────────────┬──────────────────────
         │ S1.1  S1.2  S1.3  S1.4  S1.5  S1.6  │ S2.1 S2.2 S2.3 S2.4 S2.5 S2.6│ S3.1…S3.7
  Scaff  │ Found Store API   Auth  CLI   Hard   │ Ver  Enc  Life  RL   Web  Rel │ Quot Repl OIDC Tier Clus
─────────┴──────────────────────────────────────┴────────────────────────────┴──────────────────────
Release    —        v0.1.0                              v0.2.0  v1.0.0             v1.1+  v1.2+
```

### Guiding Principles for Execution

- **Vertical slices first**: deliver a working end-to-end feature (API + storage + test) before moving on
- **No orphaned code**: every package written has unit tests and is wired into the binary before the sprint closes
- **Compatibility locks in early**: S3 wire protocol correctness is non-negotiable from Sprint 1.3 onward
- **Security is not a phase**: TLS and auth are Phase 1 features, not Phase 2 polish
- **Community checkpoints**: publish release notes and a compatibility matrix at every version tag

---

## 2. Phase 0 — Project Scaffolding

**Goal:** An empty but correctly structured repository that compiles, lints, and has CI running.
**Exit criteria:** `make build` produces a binary; `make lint` and `make test` pass; CI is green on `main`.

### Tasks

| # | Task | Owner hint | Files touched |
|---|---|---|---|
| P0-1 | Initialize `go.mod` with module path `github.com/madhavkobal/sangraha`, Go 1.22 | backend | `go.mod` |
| P0-2 | Create full directory skeleton per CLAUDE.md §3 | backend | all dirs |
| P0-3 | Write `cmd/sangraha/main.go` stub — prints version and exits | backend | `cmd/sangraha/main.go` |
| P0-4 | Write `Makefile` with targets: `build`, `build-all`, `test`, `lint`, `clean`, `web`, `release` | devops | `Makefile` |
| P0-5 | Add `golangci-lint` config (`.golangci.yml`) with linters from CLAUDE.md §13 | devops | `.golangci.yml` |
| P0-6 | Add `.github/workflows/ci.yml`: vet → lint → test → build | devops | `.github/workflows/ci.yml` |
| P0-7 | Add `.github/workflows/release.yml`: triggered on `v*` tags | devops | `.github/workflows/release.yml` |
| P0-8 | Add `LICENSE` (Apache 2.0 recommended for open-source storage tooling) | project | `LICENSE` |
| P0-9 | Write initial `README.md` with project description, install, and quick-start placeholders | docs | `README.md` |
| P0-10 | Add `scripts/gen-certs.sh` for dev TLS cert generation | devops | `scripts/gen-certs.sh` |
| P0-11 | Configure `dependabot.yml` for Go and npm dependency updates | devops | `.github/dependabot.yml` |

### Acceptance Criteria

- `go build ./cmd/sangraha` succeeds with zero warnings
- `golangci-lint run` exits 0 on the stub codebase
- `go test ./...` exits 0 (no tests yet, but must not error)
- GitHub Actions CI workflow runs and passes on push to `main`
- Release workflow is present and syntactically valid (dry-run mode)

### Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Module path conflicts if repo is renamed | Low | Lock module path in `go.mod` early; document in CLAUDE.md |
| CI runner permissions for artifact upload | Medium | Test release workflow with a `v0.0.0-test` tag before Phase 1 |

---

## 3. Phase 1 — Functional MVP

**Goal:** A working S3-compatible server that any AWS SDK can talk to, with local filesystem storage, SigV4 authentication, TLS, and a minimal CLI.
**Target version:** `v0.1.0`
**Exit criteria:** All Sprint 1.x acceptance criteria pass; integration tests using `minio-go` pass against a live binary; `aws s3` CLI can create buckets, upload, download, and delete objects.

---

### Sprint 1.1 — Foundation

**Goal:** Config loading, metadata store, and dependency injection wiring.

#### Tasks

| # | Task | Package | Notes |
|---|---|---|---|
| 1.1-1 | Define `Config` struct with all fields from CLAUDE.md §11 | `internal/config/config.go` | Use `viper` for YAML + env var parsing |
| 1.1-2 | Write `defaults.go` with sensible fallback values | `internal/config/defaults.go` | S3 port 9000, admin 9001, data dir `~/.sangraha/data` |
| 1.1-3 | Define `MetadataStore` interface | `internal/metadata/store.go` | Methods: `PutBucket`, `GetBucket`, `DeleteBucket`, `ListBuckets`, `PutObject`, `GetObject`, `DeleteObject`, `ListObjects` |
| 1.1-4 | Implement bbolt metadata store | `internal/metadata/bbolt/bbolt.go` | Three buckets: `__buckets`, `__objects`, `__uploads` |
| 1.1-5 | Write unit tests for bbolt store (CRUD + list pagination) | `internal/metadata/bbolt/bbolt_test.go` | Table-driven; use `t.TempDir()` |
| 1.1-6 | Define `Backend` interface | `internal/backend/interface.go` | `Write`, `Read`, `Delete`, `Exists`, `Stat` — do not change after this sprint |
| 1.1-7 | Implement local filesystem backend | `internal/backend/localfs/localfs.go` | Path: `<data-dir>/<bucket>/<key>`; streaming I/O; create parent dirs atomically |
| 1.1-8 | Write unit tests for localfs backend | `internal/backend/localfs/localfs_test.go` | Test Write+Read roundtrip, Delete, Exists, Stat, path traversal rejection |
| 1.1-9 | Write `StorageEngine` struct wiring config → backend → metadata | `internal/storage/engine.go` | Constructor: `NewStorageEngine(cfg, backend, meta)` |

#### Acceptance Criteria

- `MetadataStore` interface compiles; bbolt implementation passes all unit tests
- `Backend` interface compiles; localfs implementation passes all unit tests including a path traversal test (`../escape` key must return `ErrInvalidKey`)
- `StorageEngine` wires together without panicking
- All packages have `≥ 80%` test coverage
- `golangci-lint` passes

#### Key Data Structures

```go
// internal/metadata/store.go
type BucketRecord struct {
    Name        string
    Region      string
    CreatedAt   time.Time
    Versioning  string   // "enabled" | "suspended" | "disabled"
    ACL         string
}

type ObjectRecord struct {
    Bucket      string
    Key         string
    VersionID   string
    ETag        string
    Size        int64
    ContentType string
    UserMeta    map[string]string
    CreatedAt   time.Time
    DeleteMarker bool
}

type UploadRecord struct {
    UploadID    string
    Bucket      string
    Key         string
    Initiated   time.Time
    Parts       []PartRecord
}
```

---

### Sprint 1.2 — Storage Core

**Goal:** Bucket and object business logic sitting above the backend and metadata interfaces.

#### Tasks

| # | Task | Package | Notes |
|---|---|---|---|
| 1.2-1 | Implement `bucket.go`: `CreateBucket`, `DeleteBucket`, `ListBuckets`, `HeadBucket` | `internal/storage/bucket.go` | Validate name (3-63 chars, lowercase, no `..`); enforce `max_bucket_count` |
| 1.2-2 | Implement `object.go`: `PutObject`, `GetObject`, `DeleteObject`, `HeadObject`, `CopyObject` | `internal/storage/object.go` | Stream data through backend; compute MD5 ETag; detect content-type via `net/http.DetectContentType` |
| 1.2-3 | Implement `ListObjectsV2` with prefix/delimiter pagination | `internal/storage/object.go` | Continuation token = base64(last-seen-key); max 1000 per page |
| 1.2-4 | Implement multipart upload state machine | `internal/storage/multipart.go` | `CreateMultipartUpload` → `UploadPart` → `CompleteMultipartUpload` / `AbortMultipartUpload`; persist state in metadata store |
| 1.2-5 | Define sentinel errors | `internal/storage/errors.go` | `ErrBucketNotFound`, `ErrBucketNotEmpty`, `ErrBucketAlreadyExists`, `ErrObjectNotFound`, `ErrKeyTooLong`, `ErrInvalidKey`, `ErrInvalidBucketName`, `ErrUploadNotFound` |
| 1.2-6 | Write comprehensive unit tests for bucket logic | `internal/storage/bucket_test.go` | Test name validation edge cases exhaustively |
| 1.2-7 | Write comprehensive unit tests for object logic | `internal/storage/object_test.go` | Test ETag correctness, content-type detection, CopyObject preserves metadata |
| 1.2-8 | Write unit tests for multipart state machine | `internal/storage/multipart_test.go` | Test part ordering, minimum part size (5MB) enforcement, ETag computation for composite upload |

#### Acceptance Criteria

- `PutObject` + `GetObject` round-trip preserves byte-for-byte content integrity (verified with SHA-256 hash comparison in test)
- Composite multipart ETag matches S3 spec: `md5(concat(part_md5s))-N` where N = part count
- `DeleteBucket` on a non-empty bucket returns `ErrBucketNotEmpty`
- Object keys with `..` segments are rejected at the storage layer
- Content-type is auto-detected when `Content-Type` header is absent
- Test coverage ≥ 80% for all `internal/storage/` packages

#### ETag Computation Rules

```
Single-part object:   ETag = '"' + hex(md5(body)) + '"'
Multipart object:     ETag = '"' + hex(md5(part1_md5 + part2_md5 + ... + partN_md5)) + '-' + N + '"'
```

---

### Sprint 1.3 — S3 API Layer

**Goal:** A real HTTP server that handles S3 protocol requests and returns correct XML responses.

#### Tasks

| # | Task | Package | Notes |
|---|---|---|---|
| 1.3-1 | Set up chi router for S3 API | `internal/api/s3/router.go` | Path patterns: `/{bucket}` and `/{bucket}/{key:.*}` |
| 1.3-2 | Implement bucket HTTP handlers | `internal/api/s3/bucket.go` | `PUT /{bucket}`, `DELETE /{bucket}`, `HEAD /{bucket}`, `GET /` |
| 1.3-3 | Implement object HTTP handlers | `internal/api/s3/object.go` | `PUT`, `GET`, `DELETE`, `HEAD` on `/{bucket}/{key}` plus `CopyObject` detection via `x-amz-copy-source` |
| 1.3-4 | Implement multipart HTTP handlers | `internal/api/s3/multipart.go` | `?uploads`, `?partNumber&uploadId`, `?uploadId` (complete/abort/list) |
| 1.3-5 | Implement `ListObjectsV2` handler | `internal/api/s3/list.go` | `GET /{bucket}?list-type=2`; also `?versions` for `ListObjectVersions` (stub for Phase 2) |
| 1.3-6 | Implement `DeleteObjects` batch handler | `internal/api/s3/object.go` | `POST /{bucket}?delete`; parse XML body; respond with `DeleteResult` XML |
| 1.3-7 | Implement XML error response builder | `internal/api/s3/errors.go` | Map storage sentinel errors → S3 error codes + HTTP status codes |
| 1.3-8 | Implement `requestid` middleware | `internal/api/middleware/requestid.go` | Generate ULID per request; inject as `X-Request-Id` and `x-amz-request-id` headers |
| 1.3-9 | Set up admin chi router with health + metrics stubs | `internal/api/admin/router.go`, `health.go`, `metrics.go` | `/healthz` returns 200; `/readyz` checks backend reachability |
| 1.3-10 | Wire both servers into `main.go` with graceful shutdown | `cmd/sangraha/main.go` | `signal.NotifyContext`; drain in-flight requests with 30s timeout |
| 1.3-11 | Write handler-level unit tests using `httptest` | `internal/api/s3/*_test.go` | Mock `StorageEngine`; test HTTP status codes, headers, response body XML |

#### Acceptance Criteria

- `PUT /bucket` returns `200 OK` with empty body (S3 spec)
- `GET /` returns `ListAllMyBucketsResult` XML with correct namespace
- `PUT /bucket/key` streams body to backend; returns `200` with `ETag` header (quoted)
- `GET /bucket/key` streams content with correct `Content-Type` and `Content-Length`
- `DELETE /bucket/key` returns `204 No Content`
- `HEAD /bucket/key` returns `200` with metadata headers, no body
- `404` responses include `NoSuchKey` or `NoSuchBucket` XML error body
- `x-amz-request-id` header present on every response
- `ListObjectsV2` returns correct XML with `IsTruncated`, `NextContinuationToken` when page is full

#### S3 Status Code Reference

| Operation | Success | Key Error Codes |
|---|---|---|
| CreateBucket | 200 | 409 BucketAlreadyExists, 400 InvalidBucketName |
| DeleteBucket | 204 | 404 NoSuchBucket, 409 BucketNotEmpty |
| HeadBucket | 200 | 404 NoSuchBucket, 403 AccessDenied |
| ListBuckets | 200 | — |
| PutObject | 200 | 404 NoSuchBucket, 400 EntityTooLarge |
| GetObject | 200 | 404 NoSuchKey, 403 AccessDenied |
| DeleteObject | 204 | 404 NoSuchKey |
| HeadObject | 200 | 404 NoSuchKey |
| CopyObject | 200 | 404 NoSuchKey (source) |
| CompleteMultipartUpload | 200 | 400 InvalidPart, 400 InvalidPartOrder |

---

### Sprint 1.4 — Auth & Security Baseline

**Goal:** SigV4 authentication, TLS, and access key management.

#### Tasks

| # | Task | Package | Notes |
|---|---|---|---|
| 1.4-1 | Implement AWS Signature Version 4 verifier | `internal/auth/sigv4.go` | Canonical request → string-to-sign → HMAC-SHA256 signing key → signature comparison; use `crypto/hmac` + `crypto/sha256` |
| 1.4-2 | Implement access key / secret key store | `internal/auth/tokens.go` | Store in bbolt; bcrypt (cost=12) secrets at rest; methods: `Create`, `Get`, `Delete`, `Rotate` |
| 1.4-3 | Implement SigV4 auth middleware | `internal/api/middleware/auth.go` | Parse `Authorization` header; look up access key; verify signature; inject `UserContext` into request context |
| 1.4-4 | Implement TLS config helper | `internal/api/middleware/tls.go` | Load cert+key from config; auto-generate self-signed ECDSA cert if files absent; enforce TLS 1.2 minimum |
| 1.4-5 | Implement `sangraha init` command | `cli/root.go` (or new `cli/init.go`) | Interactive wizard: set data dir, generate certs, create root access key + secret; print secret once |
| 1.4-6 | Implement Admin API bearer token auth | `internal/api/middleware/auth.go` | HMAC-SHA256 signed JWT; verify on every admin request |
| 1.4-7 | Write unit tests for SigV4 | `internal/auth/sigv4_test.go` | Use AWS SigV4 test vectors from AWS documentation |
| 1.4-8 | Write unit tests for TLS helper | `internal/api/middleware/tls_test.go` | Verify self-signed cert is generated when absent; verify TLS 1.2 minimum is enforced |
| 1.4-9 | Ensure no secrets appear in logs | everywhere | Code review checklist item; add `gosec` scan to CI |

#### Acceptance Criteria

- Requests without a valid `Authorization` header return `403 AccessDenied` XML
- SigV4 test vectors from AWS pass
- Auto-generated self-signed cert uses ECDSA P-256 (not RSA 2048) for performance
- TLS 1.0 and 1.1 are rejected (verified with `openssl s_client -tls1_1`)
- `sangraha init` generates a unique access key + secret and prints secret exactly once to stdout
- Secrets stored in bbolt are bcrypt-hashed (verified by reading raw bytes from db)
- `gosec` scan exits 0 (no HIGH severity findings)

#### SigV4 Implementation Notes

```
Canonical Request:
  HTTPMethod\n
  CanonicalURI\n
  CanonicalQueryString\n
  CanonicalHeaders\n
  SignedHeaders\n
  HexEncode(Hash(Payload))

String to Sign:
  "AWS4-HMAC-SHA256\n" + date + "\n" + scope + "\n" + HexEncode(Hash(CanonicalRequest))

Signing Key:
  HMAC(HMAC(HMAC(HMAC("AWS4"+SecretKey, date), region), service), "aws4_request")
```

Critical edge cases to handle:
- Presigned URL auth (query-string parameters instead of header) — stub in Phase 1, full in Phase 2
- Chunked transfer encoding (`x-amz-content-sha256: STREAMING-AWS4-HMAC-SHA256-PAYLOAD`)
- Virtual-hosted style vs. path style bucket addressing

---

### Sprint 1.5 — CLI & Observability

**Goal:** A usable command-line interface and production-grade observability.

#### Tasks

| # | Task | Package | Notes |
|---|---|---|---|
| 1.5-1 | Implement `cli/root.go` — cobra root command, global flags (`--server`, `--access-key`, `--secret-key`, `--json`) | `cli/root.go` | Env var fallback for all flags |
| 1.5-2 | Implement `sangraha server start/stop/status` | `cli/server.go` | `start` runs the HTTP servers; `--daemon` forks and writes PID file; `stop` sends SIGTERM via PID file |
| 1.5-3 | Implement `sangraha bucket create/delete/list` | `cli/bucket.go` | `list` outputs aligned table; `--json` outputs JSON array |
| 1.5-4 | Implement `sangraha object put/get/delete/list/cp/mv` | `cli/object.go` | `put` reads from file or stdin; `get` writes to file or stdout; `cp`/`mv` support `s3://bucket/key` URIs |
| 1.5-5 | Implement `sangraha user create/delete/list/rotate-key` | `cli/user.go` | `create` prints access key + secret once; never logs the secret |
| 1.5-6 | Implement `sangraha config show/set/validate` | `cli/config.go` | `validate` parses the config file and reports all errors at once |
| 1.5-7 | Implement `sangraha admin gc/export/import` | `cli/admin.go` | `gc` calls `POST /admin/v1/gc`; `export`/`import` are Phase 1 stubs that print "not yet implemented" |
| 1.5-8 | Wire Prometheus metrics into middleware | `internal/api/middleware/`, `internal/api/admin/metrics.go` | Counters: `sangraha_requests_total{method,bucket,status}`; histograms: `sangraha_request_duration_seconds`, `sangraha_bytes_transferred_total` |
| 1.5-9 | Implement structured zerolog logging throughout | everywhere | JSON format; include `request_id`, `method`, `path`, `status`, `duration_ms`, `user` on every request log line |
| 1.5-10 | Implement `/admin/v1/info` endpoint | `internal/api/admin/` | Returns: version, build time, uptime, backend type, object count, total bytes used |
| 1.5-11 | Write CLI integration smoke tests | `test/integration/cli_test.go` | Start server; run CLI commands; assert output |

#### Acceptance Criteria

- `sangraha bucket list --json` outputs valid JSON parseable by `jq`
- `sangraha object put mybucket mykey ./file && sangraha object get mybucket mykey` round-trips the file
- `sangraha object cp s3://src/key s3://dst/key` performs a server-side copy
- Prometheus metrics are scraped at `/admin/v1/metrics` in text format
- Every request log line contains `request_id` and `duration_ms`
- `--json` flag is respected by all list commands
- Exit code 3 is returned when bucket or object is not found

---

### Sprint 1.6 — MVP Hardening & Release

**Goal:** Integration tests pass; S3 compatibility verified; documentation complete; `v0.1.0` tagged.

#### Tasks

| # | Task | Notes |
|---|---|---|
| 1.6-1 | Write integration tests using `minio-go` | Cover: CreateBucket, PutObject, GetObject, DeleteObject, HeadObject, CopyObject, ListObjectsV2, multipart upload (3-part), DeleteObjects |
| 1.6-2 | Write `scripts/integration-test.sh` | Start binary on ports 19000/19001, run tests, tear down, report |
| 1.6-3 | Add integration test CI job | Run after unit test job; use built binary artifact |
| 1.6-4 | Fix all S3 compatibility issues found during integration testing | — |
| 1.6-5 | Run Ceph s3-tests subset (bucket and object operations) | Document which tests pass/fail in `docs/s3-compatibility.md` |
| 1.6-6 | Performance baseline: measure PUT/GET throughput for 4KB and 1MB objects | Document in `docs/performance.md` |
| 1.6-7 | Write `docs/config-reference.md` | Every config key, type, default, and env var override |
| 1.6-8 | Write getting-started section of `README.md` | Install, init, server start, first bucket + object via CLI and AWS SDK |
| 1.6-9 | Cross-compile for all target platforms | linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 |
| 1.6-10 | Tag `v0.1.0` and publish GitHub release | Include SHA256SUMS; write release notes |

#### Acceptance Criteria

- All `minio-go` integration tests pass
- `aws s3 mb s3://test && aws s3 cp file s3://test/key && aws s3 cp s3://test/key /tmp/out` succeeds against running sangraha binary
- S3 compatibility report documents ≥ 90% pass rate on the tested subset
- Binary sizes: linux/amd64 < 25MB (before `upx`, web assets included)
- `v0.1.0` release is published with all platform binaries and SHA256SUMS

---

## 4. Phase 2 — Production Hardening

**Goal:** A server that can be deployed in a real team environment with access controls, encryption, lifecycle management, rate limiting, audit logging, and a web dashboard.
**Target version:** `v1.0.0`
**Exit criteria:** All Phase 2 sprints complete; web dashboard renders; encryption round-trips verified; `aws s3api` versioning commands work; rate limiting verified under load.

---

### Sprint 2.1 — Versioning & ACLs

**Goal:** Per-bucket object versioning and canned ACL enforcement.

#### Tasks

| # | Task | Package | Notes |
|---|---|---|---|
| 2.1-1 | Implement per-bucket versioning toggle | `internal/storage/version.go`, `internal/api/s3/bucket.go` | States: `disabled` (default), `enabled`, `suspended`; add `PUT /{bucket}?versioning` handler |
| 2.1-2 | Modify `PutObject` to assign version IDs when versioning enabled | `internal/storage/object.go` | Version ID = ULID; stored in metadata; `x-amz-version-id` header in response |
| 2.1-3 | Implement `DeleteObject` version-aware behavior | `internal/storage/object.go` | Without version ID: insert delete marker; with version ID: permanently delete that version |
| 2.1-4 | Implement `ListObjectVersions` handler | `internal/api/s3/list.go` | `GET /{bucket}?versions`; return `ListVersionsResult` XML with Version and DeleteMarker entries |
| 2.1-5 | Implement `GetObject` with `?versionId=` query param | `internal/api/s3/object.go` | Retrieve specific version |
| 2.1-6 | Implement canned ACL evaluation | `internal/auth/acl.go` | `private`, `public-read`, `public-read-write`, `authenticated-read`; evaluated after SigV4 auth |
| 2.1-7 | Add ACL support to `CreateBucket` and `PutObject` | handlers | Parse `x-amz-acl` header; store in metadata; enforce on `GetObject` |
| 2.1-8 | Implement `GetBucketAcl` / `PutBucketAcl` handlers | `internal/api/s3/bucket.go` | Return/accept `AccessControlPolicy` XML |
| 2.1-9 | Write unit + integration tests for versioning | — | Test: enable versioning, put 3 versions, list all, get specific version, delete marker behavior |
| 2.1-10 | Update `sangraha bucket versioning` CLI command | `cli/bucket.go` | Now functional; calls `PUT /{bucket}?versioning` |

#### Acceptance Criteria

- `aws s3api put-bucket-versioning --versioning-configuration Status=Enabled` works
- Three successive PUTs of the same key produce three distinct version IDs
- DELETE without version ID creates a delete marker; object is invisible to `GetObject` but visible in `ListObjectVersions`
- DELETE with version ID permanently removes that version
- `public-read` bucket allows unauthenticated `GetObject`; `private` bucket returns `403`

---

### Sprint 2.2 — Encryption & Presigned URLs

**Goal:** Optional server-side encryption and presigned URL support.

#### Tasks

| # | Task | Package | Notes |
|---|---|---|---|
| 2.2-1 | Implement AES-256-GCM server-side encryption | `internal/storage/object.go` | Encrypt stream on `PutObject`; decrypt stream on `GetObject`; per-object random nonce; key derivation via HKDF from master key |
| 2.2-2 | Store encryption metadata per object | `internal/metadata/store.go` | Fields: `EncryptionAlgo`, `Nonce`, `WrappedKey`; never store plaintext key |
| 2.2-3 | Add `x-amz-server-side-encryption: AES256` header support | `internal/api/s3/object.go` | Per-request opt-in; or auto-encrypt if bucket default encryption is set |
| 2.2-4 | Implement `PutBucketEncryption` / `GetBucketEncryption` | `internal/api/s3/bucket.go` | Store default SSE config in bucket record |
| 2.2-5 | Implement presigned URL generation | `internal/auth/sigv4.go` | `X-Amz-Signature` in query string; `X-Amz-Expires` max 604800s (7 days); verify on arrival |
| 2.2-6 | Implement presigned URL verification middleware | `internal/api/middleware/auth.go` | Detect presigned URL (presence of `X-Amz-Signature` query param); validate expiry and signature |
| 2.2-7 | Implement `sangraha object presign` CLI command | `cli/object.go` | `--expires` flag; prints URL to stdout |
| 2.2-8 | Write encryption round-trip tests | `internal/storage/object_test.go` | Verify ciphertext differs from plaintext; verify decrypted == original |
| 2.2-9 | Write presigned URL tests | `test/integration/presign_test.go` | Generate URL; fetch with plain `http.Get`; verify body |

#### Acceptance Criteria

- Objects stored with SSE are unreadable if bbolt metadata is deleted (nonce is gone)
- Presigned GET URL works without `Authorization` header
- Presigned URL expires correctly (403 after expiry)
- `x-amz-server-side-encryption: AES256` header present in PutObject response when encryption active
- Encryption does not break ETag calculation (ETag is of plaintext; encryption happens transparently)

---

### Sprint 2.3 — Lifecycle, Tagging & CORS

**Goal:** Object tagging, bucket lifecycle rules, and CORS support for browser clients.

#### Tasks

| # | Task | Package | Notes |
|---|---|---|---|
| 2.3-1 | Implement object tagging — `PutObjectTagging`, `GetObjectTagging`, `DeleteObjectTagging` | `internal/api/s3/object.go` | Tags stored in object metadata; max 10 tags per object; `TagSet` XML |
| 2.3-2 | Implement `PutBucketLifecycleConfiguration` / `GetBucketLifecycleConfiguration` | `internal/api/s3/bucket.go` | Parse S3 lifecycle XML; store rules in bucket metadata |
| 2.3-3 | Implement lifecycle rule engine | `internal/storage/lifecycle.go` | Background goroutine runs every configurable interval (default: 1h); evaluate expiration rules; delete expired objects; emit log line per deletion |
| 2.3-4 | Support tag-based lifecycle filters | `internal/storage/lifecycle.go` | Filter rules by tag key/value; AND logic for multiple tags |
| 2.3-5 | Implement `PutBucketCors` / `GetBucketCors` / `DeleteBucketCors` | `internal/api/s3/bucket.go` | Store CORS config in bucket metadata |
| 2.3-6 | Implement CORS middleware | `internal/api/middleware/` | On OPTIONS preflight: check `Origin` against bucket CORS rules; set `Access-Control-*` headers |
| 2.3-7 | Write unit tests for lifecycle engine | `internal/storage/lifecycle_test.go` | Use a fake clock; verify objects are deleted after simulated time advance |
| 2.3-8 | Write integration tests for tagging | `test/integration/tagging_test.go` | Put, get, delete tags; verify tag-based lifecycle |

#### Acceptance Criteria

- `aws s3api put-bucket-lifecycle-configuration` stores rules; background task deletes expired objects
- CORS preflight returns correct `Access-Control-Allow-Origin` for matching origins
- Browser `fetch()` to sangraha S3 port works after CORS is configured
- Lifecycle engine does not delete objects while an in-progress multipart upload references them
- Object tags survive CopyObject by default

---

### Sprint 2.4 — Rate Limiting & Audit Log

**Goal:** Per-key rate limiting and a complete, searchable audit trail.

#### Tasks

| # | Task | Package | Notes |
|---|---|---|---|
| 2.4-1 | Implement token bucket rate limiter | `internal/api/middleware/ratelimit.go` | Per access key and per source IP; configurable RPS and burst from config; use `golang.org/x/time/rate` |
| 2.4-2 | Return `429 Too Many Requests` with `Retry-After` header on limit breach | `internal/api/middleware/ratelimit.go` | S3 error code: `SlowDown` |
| 2.4-3 | Implement audit event struct and emitter | `internal/audit/audit.go` | Fields per CLAUDE.md §7; write to append-only file; configurable file path |
| 2.4-4 | Wire audit middleware to emit start + complete events | `internal/api/middleware/audit.go` | Start event: method, path, user, IP; complete event: adds status, bytes, duration |
| 2.4-5 | Implement audit log query endpoint | `internal/api/admin/` | `GET /admin/v1/audit?from=&to=&user=&bucket=&action=&limit=`; parse audit log; filter; return JSON |
| 2.4-6 | Add syslog forwarding option | `internal/audit/audit.go` | Config: `audit_log: syslog://localhost:514`; fall back to local file if syslog unreachable |
| 2.4-7 | Write rate limiter tests | `internal/api/middleware/ratelimit_test.go` | Burst 10 req/s limit; send 15 requests; verify 10 succeed and 5 get 429 |
| 2.4-8 | Write audit log tests | `internal/audit/audit_test.go` | Emit events; read back; verify JSON structure; verify append-only (no overwrites) |

#### Acceptance Criteria

- Rate limiter correctly limits to configured RPS per access key
- Audit log entry is written for every request, including 4xx and 5xx responses
- Audit log file cannot be truncated by the server process (opened with `os.O_APPEND`)
- Audit query endpoint filters by all supported fields
- `gosec` scan still exits 0

---

### Sprint 2.5 — Web Dashboard

**Goal:** An embedded React SPA served on the admin port for visual management.

#### Tasks

| # | Task | Notes |
|---|---|---|
| 2.5-1 | Bootstrap React 18 + TypeScript + Vite project in `web/` | `npm create vite@latest` with React+TS template |
| 2.5-2 | Configure Tailwind CSS and shadcn/ui | Install and configure per shadcn docs; dark mode via `class` strategy |
| 2.5-3 | Implement typed API client in `web/src/api/` | Thin `fetch` wrappers over Admin REST API; typed request/response DTOs |
| 2.5-4 | Implement **Dashboard page** | Storage usage gauge (bytes used / total); request rate chart (Prometheus data polled every 30s); top-5 buckets by size |
| 2.5-5 | Implement **Buckets page** | List all buckets with size, object count, versioning status; create bucket dialog; delete bucket with typed confirmation modal |
| 2.5-6 | Implement **Objects page** | Breadcrumb object browser; prefix navigation; file upload (drag-and-drop + button); download; delete; metadata viewer sidebar |
| 2.5-7 | Implement **Users page** | List users; create user (shows access key + secret once in modal); delete user; rotate key |
| 2.5-8 | Implement **Audit Log page** | Virtualized table (TanStack Virtual); filter by date range, user, bucket, action; export to CSV |
| 2.5-9 | Implement **Settings page** | Read-only view of current server config (from `/admin/v1/info`); TLS status; log level selector (calls admin API) |
| 2.5-10 | Add login page with bearer token auth | JWT stored in `sessionStorage`; auto-redirect to login on 401 |
| 2.5-11 | Wire `//go:embed` to include `web/dist` in binary | `internal/web/embed.go`; serve from admin chi router under `/` |
| 2.5-12 | Add `make web` to Makefile and wire into CI | Run `npm run build` before Go build |
| 2.5-13 | Write Playwright E2E tests for critical flows | Create bucket, upload object, verify in browser, delete |

#### Acceptance Criteria

- Dashboard loads with no external CDN requests (verified with browser devtools Network tab)
- File upload works for files up to 100MB (uses multipart API for large files)
- Confirmation modal requires typing the bucket/object name to proceed with delete
- Dark mode is default; respects `prefers-color-scheme`
- All API errors display inline with the raw error code
- Binary size increase from embedding web assets < 5MB (Vite tree-shaking + minification)

---

### Sprint 2.6 — Production Release

**Goal:** `v1.0.0` — production-ready, documented, and publicly announced.

#### Tasks

| # | Task | Notes |
|---|---|---|
| 2.6-1 | Full regression test run (unit + integration + E2E) | Fix all failures |
| 2.6-2 | Complete S3 compatibility report | Run full Ceph s3-tests suite; document pass/fail per operation; target ≥ 95% pass rate |
| 2.6-3 | Security review | Check all `gosec` findings; review TLS config; verify no secrets in logs; check path traversal coverage |
| 2.6-4 | Performance benchmarks | Document single-node throughput for 4KB, 64KB, 1MB, 1GB objects; compare to Phase 1 baseline |
| 2.6-5 | Write `docs/operations-guide.md` | Deployment, backup, restore, upgrade, log rotation, TLS cert renewal |
| 2.6-6 | Write `docs/security-guide.md` | Key rotation, ACL strategy, audit log management, TLS hardening |
| 2.6-7 | Write `docs/s3-compatibility.md` | Tested S3 operations; known gaps; workarounds |
| 2.6-8 | Write migration guide from Phase 1 | Config file changes; new required fields |
| 2.6-9 | Publish Docker image to GHCR | `ghcr.io/madhavkobal/sangraha:1.0.0` |
| 2.6-10 | Tag `v1.0.0`, publish GitHub release | Changelog, SHA256SUMS, all platform binaries |

---

## 5. Phase 3 — Scale & Enterprise

**Goal:** Horizontal scalability, enterprise authentication, storage tiering, and event-driven integrations.
**Target versions:** `v1.1.0` through `v2.0.0`
**Exit criteria:** Multi-node cluster operates correctly under failure scenarios; OIDC SSO login works; storage tiering moves objects between tiers transparently.

---

### Sprint 3.1 — Quotas & Event Notifications

**Goal:** Per-bucket and per-user storage quotas; S3-compatible event notifications via webhooks.

#### Tasks

| # | Task | Package | Notes |
|---|---|---|---|
| 3.1-1 | Implement per-bucket storage quota | `internal/storage/bucket.go` | Config + admin API; enforce on `PutObject`; return `QuotaExceeded` (custom error, map to 507) |
| 3.1-2 | Implement per-user storage quota | `internal/auth/iam.go` | Sum bytes across all buckets owned by user; enforce on write |
| 3.1-3 | Add quota endpoints to admin API | `internal/api/admin/buckets.go` | `GET /admin/v1/buckets/{name}/quota`, `PUT /admin/v1/buckets/{name}/quota` |
| 3.1-4 | Implement `PutBucketNotificationConfiguration` handler | `internal/api/s3/bucket.go` | Parse S3 notification XML; store webhook URL + event filter in bucket metadata |
| 3.1-5 | Implement event notification dispatcher | `internal/storage/notification.go` | Background goroutine; non-blocking channel; retry with exponential backoff; payload: S3-compatible event JSON |
| 3.1-6 | Emit events on `PutObject`, `DeleteObject`, `CopyObject` | `internal/storage/object.go` | Respect event filter (prefix, suffix, event type) |
| 3.1-7 | Write quota enforcement tests | — | Test: quota at 100 bytes; PUT 90-byte object succeeds; PUT 20-byte object fails with 507 |
| 3.1-8 | Write webhook delivery tests | — | Start a test HTTP server; configure as webhook target; assert event received within 5s |

#### Event Notification Payload (S3-compatible)

```json
{
  "Records": [{
    "eventVersion": "2.1",
    "eventSource": "sangraha:s3",
    "eventTime": "2026-03-04T12:00:00Z",
    "eventName": "ObjectCreated:Put",
    "s3": {
      "bucket": { "name": "my-bucket" },
      "object": { "key": "photos/cat.jpg", "size": 204800, "eTag": "abc123" }
    }
  }]
}
```

---

### Sprint 3.2 — Static Website Hosting

**Goal:** Serve a bucket as a static website.

#### Tasks

| # | Task | Notes |
|---|---|---|
| 3.2-1 | Implement `PutBucketWebsite` / `GetBucketWebsite` / `DeleteBucketWebsite` | Store website config (index doc, error doc) in bucket metadata |
| 3.2-2 | Implement website serving mode on the S3 port | When `?website` subdomain or header detected, serve `index.html` for directory paths; serve error document on 404 |
| 3.2-3 | Add virtual-host style bucket addressing | `<bucket>.s3.<host>` → route to bucket; required for website hosting |
| 3.2-4 | Add website hosting to CLI and web dashboard | `sangraha bucket website set/get/delete`; Settings tab in web UI |
| 3.2-5 | Write integration tests | Upload an HTML/CSS/JS site; verify browser navigation works |

---

### Sprint 3.3 — Object Replication

**Goal:** Asynchronous object replication to a secondary sangraha instance or upstream S3/GCS.

#### Tasks

| # | Task | Notes |
|---|---|---|
| 3.3-1 | Implement `PutBucketReplication` / `GetBucketReplication` | Per-bucket replication rules: destination URL, access key, prefix filter |
| 3.3-2 | Implement async replication dispatcher | Worker pool; uses `minio-go` (for S3 targets) or sangraha S3 API (for sangraha targets); tracks replication status per object |
| 3.3-3 | Add replication status to object metadata | `x-amz-replication-status: PENDING | COMPLETE | FAILED` |
| 3.3-4 | Implement replication status API | `GET /admin/v1/replication/status` — lag, error rate, queue depth |
| 3.3-5 | Handle replication failures with DLQ | Dead-letter queue persisted in bbolt; admin can inspect and retry |
| 3.3-6 | Write replication integration tests | Start two sangraha instances; configure replication; PUT object to source; assert appears on destination within 10s |

---

### Sprint 3.4 — External Authentication (OIDC/LDAP)

**Goal:** SSO login via OIDC (Keycloak, Okta, Auth0, GitHub) and LDAP group-to-policy mapping.

#### Tasks

| # | Task | Notes |
|---|---|---|
| 3.4-1 | Implement OIDC token exchange for admin API | Admin login page redirects to IdP; exchanges code for JWT; validates JWT against JWKS endpoint |
| 3.4-2 | Map OIDC claims to internal roles | Config: `oidc.role_claim: "sangraha_role"`; values: `admin`, `user`; fallback to `user` |
| 3.4-3 | Implement LDAP bind for authentication | Config: `ldap.url`, `ldap.bind_dn`, `ldap.user_filter`, `ldap.group_filter`; groups → policies |
| 3.4-4 | Update admin login page for OIDC | "Login with SSO" button; handle callback; store JWT in sessionStorage |
| 3.4-5 | Write OIDC integration tests | Use a test OIDC provider (e.g., `dex` running in CI) |
| 3.4-6 | Document OIDC configuration in `docs/security-guide.md` | Keycloak, Okta, GitHub examples |

---

### Sprint 3.5 — Storage Tiering

**Goal:** Transparent hot/warm/cold storage tiering with per-bucket backend assignment.

#### Tasks

| # | Task | Notes |
|---|---|---|
| 3.5-1 | Extend `Backend` interface with `Tier()` method | Returns `Hot | Warm | Cold`; no-op for localfs (always Hot) |
| 3.5-2 | Implement S3-proxy backend | Forwards reads/writes to upstream S3/GCS/Azure using their respective SDKs; useful as a cold tier |
| 3.5-3 | Implement tier routing in StorageEngine | Config per bucket: `hot_backend: localfs`, `cold_backend: s3proxy`; lifecycle rules trigger migration |
| 3.5-4 | Implement `RestoreObject` handler | S3-compatible `POST /{bucket}/{key}?restore`; move object from cold tier to hot tier; response includes restore status |
| 3.5-5 | Implement transparent read from cold tier | On `GetObject`, if object is on cold tier, stream directly from cold backend (no hot copy unless explicitly restored) |
| 3.5-6 | Implement tier migration lifecycle action | Lifecycle rule action `Transition` moves objects to cold tier after N days |
| 3.5-7 | Update web dashboard with tier status | Object browser shows tier badge (Hot/Warm/Cold); restore button for cold objects |

---

### Sprint 3.6 — Multi-node Clustering

**Goal:** Horizontally scalable, fault-tolerant cluster with Raft-based coordination.

> This is the most complex sprint. Treat it as a sub-project with its own planning.

#### Tasks

| # | Task | Notes |
|---|---|---|
| 3.6-1 | Evaluate and choose consensus library | Options: `hashicorp/raft` (battle-tested) vs `etcd/raft` (lower-level); recommend `hashicorp/raft` for time-to-market |
| 3.6-2 | Extract metadata store to distributed backend | Implement a Raft FSM that replicates all metadata mutations; local bbolt remains as Raft log storage |
| 3.6-3 | Implement cluster membership management | `sangraha cluster join/leave/status`; gossip protocol for member discovery (use `hashicorp/memberlist`) |
| 3.6-4 | Implement leader election | Only leader handles writes; any node handles reads; redirect writes to leader if not leader |
| 3.6-5 | Implement object storage on shared or replicated backend | Option A: shared NFS/Ceph mountpoint; Option B: object replication (Sprint 3.3) between all nodes |
| 3.6-6 | Implement split-brain protection | Quorum write; reject writes if quorum is lost; document behavior |
| 3.6-7 | Update health/ready endpoints for cluster state | `/readyz` returns 503 if node is not in quorum |
| 3.6-8 | Write chaos tests | Kill leader node; verify new leader elected within 5s; verify no data loss; verify clients retry and succeed |
| 3.6-9 | Document cluster deployment in `docs/clustering.md` | 3-node and 5-node topologies; network requirements; backup strategy |

#### Architecture Change for Clustering

```
Single Node:                 Cluster:
┌────────────────┐           ┌──────┐  ┌──────┐  ┌──────┐
│  sangraha      │           │  N1  │  │  N2  │  │  N3  │
│  ┌──────────┐  │           │(lead)│  │      │  │      │
│  │  bbolt   │  │    →      └──┬───┘  └──┬───┘  └──┬───┘
│  └──────────┘  │              │          │          │
└────────────────┘           ┌──▼──────────▼──────────▼──┐
                             │       Raft consensus        │
                             │  (replicated metadata log)  │
                             └─────────────────────────────┘
                                  Shared object storage
                              (NFS / Ceph / S3-compatible)
```

---

### Sprint 3.7 — Enterprise Release

#### Tasks

| # | Task | Notes |
|---|---|---|
| 3.7-1 | Full test suite across all Phase 3 features | — |
| 3.7-2 | Third-party security audit | Engage external firm for penetration test; fix all critical/high findings |
| 3.7-3 | Performance benchmarks for cluster | 3-node write throughput; read scalability; failover latency |
| 3.7-4 | Write `docs/clustering.md`, `docs/tiering.md`, `docs/oidc.md` | — |
| 3.7-5 | Tag `v2.0.0`, publish GitHub release | Full changelog; migration guide from v1.x |

---

## 6. Cross-Cutting Concerns

These are not phase-specific — they apply throughout all phases and must be continuously maintained.

### Documentation

| Artifact | Owner | Update trigger |
|---|---|---|
| `CLAUDE.md` | All | Any architectural decision; new conventions |
| `PLAN.md` (this file) | All | Sprint completion; new tasks discovered |
| `README.md` | All | Every release |
| `docs/config-reference.md` | Backend | Any config schema change |
| `docs/s3-compatibility.md` | Backend | Any new S3 operation added or changed |
| `docs/architecture/ADR-*.md` | Lead | Any significant design decision |
| OpenAPI spec in `docs/api/` | Backend | Any admin API change |

### Architecture Decision Records (ADRs)

Create an ADR in `docs/architecture/` for every significant decision. Template:

```markdown
# ADR-NNN: <short title>
Date: YYYY-MM-DD
Status: Proposed | Accepted | Deprecated
## Context
## Decision
## Consequences
```

Required ADRs to write before Phase 1 code is written:
- ADR-001: Choice of chi over gorilla/mux
- ADR-002: bbolt as metadata store
- ADR-003: Local filesystem as default backend
- ADR-004: SigV4 as the only authentication mechanism for S3 API
- ADR-005: Embedded web dashboard vs. separate service

### Community & Governance

| Activity | Cadence | Notes |
|---|---|---|
| Release notes | Every version tag | Include: new features, bug fixes, breaking changes, migration guide |
| Security advisories | As needed | Use GitHub Security Advisories; CVE assignment if warranted |
| CONTRIBUTING.md | Phase 0 | DCO sign-off; PR template; issue templates |
| Good first issue labeling | Ongoing | Label Phase 1 test gaps and docs as `good-first-issue` |
| Changelog | Ongoing | Keep `CHANGELOG.md` in Keep a Changelog format |

---

## 7. Dependency Graph

```
Phase 0 (Scaffolding)
    │
    ▼
Sprint 1.1 (Foundation: config, metadata store, backend interface)
    │
    ▼
Sprint 1.2 (Storage Core: bucket + object business logic)
    │
    ├──────────────────────────────┐
    ▼                              ▼
Sprint 1.3 (S3 API Layer)    Sprint 1.4 (Auth & TLS)
    │                              │
    └──────────────┬───────────────┘
                   ▼
              Sprint 1.5 (CLI + Observability)
                   │
                   ▼
              Sprint 1.6 (MVP Hardening → v0.1.0)
                   │
          ┌────────┼────────┐
          ▼        ▼        ▼
       S2.1      S2.2     S2.3       (can run in parallel)
    (Versioning)(Encrypt)(Lifecycle)
          │        │        │
          └────────┼────────┘
                   ▼
              Sprint 2.4 (Rate Limiting + Audit)
                   │
                   ▼
              Sprint 2.5 (Web Dashboard)
                   │
                   ▼
              Sprint 2.6 (Production → v1.0.0)
                   │
          ┌────────┼────────┐
          ▼        ▼        ▼
       S3.1      S3.2     S3.3       (can run in parallel)
    (Quotas)  (Static) (Replicate)
       S3.4      S3.5
     (OIDC)   (Tiering)
          │
          ▼
       S3.6 (Clustering) ← depends on S3.3 (Replication) for object distribution
          │
          ▼
       S3.7 (Enterprise → v2.0.0)
```

---

## 8. Risk Register

| ID | Risk | Phase | Likelihood | Impact | Mitigation |
|---|---|---|---|---|---|
| R01 | SigV4 implementation bugs break SDK compatibility | 1 | High | High | Use AWS test vectors; test with `minio-go`, `boto3`, and `aws-sdk-go` against the same binary |
| R02 | bbolt performance bottleneck at high concurrency | 1–2 | Medium | High | bbolt uses a single write lock; benchmark early; design metadata schema to minimize write transactions; Badger backend as escape hatch |
| R03 | ETag mismatch for multipart objects breaks checksums | 1 | Medium | High | Implement multipart ETag per spec; add specific unit test comparing with S3-generated ETags |
| R04 | Path traversal vulnerability in object key handling | 1 | Low | Critical | Validate at multiple layers: storage engine + localfs backend; fuzz test with `go-fuzz` |
| R05 | AES-256-GCM nonce reuse (catastrophic) | 2 | Low | Critical | Use `crypto/rand` for nonce; never reuse; nonce stored in metadata |
| R06 | Web dashboard introduces XSS via object key display | 2 | Medium | High | Sanitize all user-controlled data in React (automatic with JSX); add CSP header; Playwright tests |
| R07 | Raft split-brain causes metadata divergence | 3 | Medium | Critical | Use quorum writes; test network partition scenarios; document behavior under partition |
| R08 | Single-binary size exceeds acceptable limit | 1–2 | Low | Medium | Monitor with `make build` output; use `upx` if needed; lazy-load web assets |
| R09 | License compatibility of dependencies | 0 | Medium | High | Audit all dependency licenses before Phase 1 release; use `go-licenses` tool |
| R10 | Breaking S3 API changes in a patch release | 2–3 | Low | High | Semantic versioning strictly enforced; S3 API changes are MAJOR bumps |

---

## 9. Definition of Done

A sprint is **done** when all of the following are true:

### Code Quality
- [ ] All tasks listed in the sprint are implemented
- [ ] `go test ./...` passes with zero failures
- [ ] `go test -race ./...` passes (no race conditions detected)
- [ ] `golangci-lint run` exits 0
- [ ] `gosec ./...` exits 0 with no HIGH severity findings
- [ ] Test coverage for new/modified packages ≥ 80%
- [ ] No `TODO` comments in production code (use GitHub issues instead)

### Documentation
- [ ] All new exported symbols have doc comments
- [ ] `CLAUDE.md` updated if any new convention was established
- [ ] `docs/config-reference.md` updated if config schema changed
- [ ] ADR written for any significant decision made during the sprint

### Integration
- [ ] New features are wired into the binary (no orphaned packages)
- [ ] New S3 operations are covered by at least one integration test
- [ ] CI pipeline is green on the feature branch
- [ ] Feature branch is rebased onto `main` with no conflicts

### Security
- [ ] No secrets appear in logs (manual review of log output during integration test)
- [ ] No `InsecureSkipVerify` introduced
- [ ] All user input validated at API boundaries

---

## 10. Versioning Strategy

| Version | Content | Upgrade path |
|---|---|---|
| `v0.1.0` | Phase 1 complete (MVP) | Fresh install only; no migration |
| `v0.2.x` | Phase 2 partial (versioning, encryption, lifecycle) | Config migration script required for encryption config |
| `v1.0.0` | Phase 2 complete (production-ready) | Minor config additions; backward-compatible |
| `v1.1.x` | Phase 3 partial (quotas, notifications, website hosting) | Backward-compatible; new config sections |
| `v1.2.x` | Replication + OIDC | New config sections; no data migration |
| `v1.3.x` | Storage tiering | New backend config; existing data stays on `localfs` |
| `v2.0.0` | Multi-node clustering | Breaking change to config schema; full migration guide required |

### Stability Guarantees

- **S3 API wire protocol**: stable from `v0.1.0`; never break without a MAJOR version bump
- **Admin REST API**: stable from `v1.0.0`; minor additions allowed in MINOR versions
- **Config file schema**: backward-compatible additions in MINOR versions; removals only in MAJOR versions
- **CLI flags**: backward-compatible additions in MINOR versions; no flag removal without deprecation notice in prior MINOR version
- **Binary data format** (bbolt schema): migration scripts provided for every MAJOR version

---

*Maintainers: update this file as sprints complete or new tasks are discovered. Sprint tasks should be reflected in GitHub Issues and Projects for trackability. When a sprint closes, add a brief retrospective note to the relevant sprint section.*
