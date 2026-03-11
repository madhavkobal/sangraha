# sangraha

> संग्रह — "collection" or "repository"

**sangraha** is a single-binary, S3-compatible object storage system built for self-hosted, on-premise, and edge deployments.

[![CI](https://github.com/madhavkobal/sangraha/actions/workflows/ci.yml/badge.svg)](https://github.com/madhavkobal/sangraha/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://go.dev)

---

## Features

- **S3-compatible API** — works with any AWS SDK, `aws s3` CLI, and S3-compatible tooling
- **Single binary** — storage engine, API server, CLI, and web UI all in one file
- **Zero runtime dependencies** — no database, broker, or cache required for single-node use
- **Embedded web console** — full administration, monitoring, and configuration UI on port 9001
- **TLS by default** — auto-generates a self-signed cert if none is provided
- **SigV4 authentication** — standard AWS signature verification on every request
- **Structured logging** — JSON logs via zerolog; Prometheus metrics at `/admin/v1/metrics`
- **Audit trail** — append-only structured log of every API call
- **Progressive scalability** — Raspberry Pi to multi-node cluster

---

## Quick Start

### Install

Download the latest binary from [Releases](https://github.com/madhavkobal/sangraha/releases):

```bash
# Linux (amd64)
curl -Lo sangraha https://github.com/madhavkobal/sangraha/releases/latest/download/sangraha-linux-amd64
chmod +x sangraha
sudo mv sangraha /usr/local/bin/
```

Or build from source:

```bash
git clone https://github.com/madhavkobal/sangraha
cd sangraha
make build          # binary at ./bin/sangraha
```

### Initialize and Start

```bash
# First-time setup: create config, generate TLS cert, create root user
sangraha init

# Start the server (S3 API on :9000, admin/web console on :9001)
sangraha server start
```

### Create a Bucket and Upload an Object

Using the sangraha CLI:

```bash
sangraha bucket create my-bucket
sangraha object put my-bucket hello.txt ./hello.txt
sangraha object get my-bucket hello.txt
sangraha bucket list
```

Using the AWS CLI:

```bash
export AWS_ACCESS_KEY_ID=<your-access-key>
export AWS_SECRET_ACCESS_KEY=<your-secret-key>
export AWS_ENDPOINT_URL=https://localhost:9000

aws s3 mb s3://my-bucket
aws s3 cp ./hello.txt s3://my-bucket/hello.txt
aws s3 ls s3://my-bucket
```

### Web Administration Console

Open `https://localhost:9001` in your browser. Log in with your access key and secret key to access:

- **Overview** — storage KPIs, request rate, latency charts
- **Buckets** — create/delete/configure buckets, lifecycle rules, CORS, versioning
- **Objects** — browse, upload (drag-and-drop), download, manage tags and versions
- **Users** — create users, rotate access keys, manage policies
- **Monitoring** — live log viewer, health status, TLS certificate management
- **Alerts** — threshold-based alert rules and history
- **Configuration** — edit, validate, and apply server configuration with diff view
- **Server** — GC, backup/restore, storage backend stats
- **Audit Log** — searchable, filterable, exportable audit trail

---

## Configuration

Default config file: `~/.sangraha/config.yaml`

```yaml
server:
  s3_address: ":9000"
  admin_address: ":9001"
  tls:
    enabled: true
    auto_self_signed: true

storage:
  backend: localfs
  data_dir: "~/.sangraha/data"

metadata:
  path: "~/.sangraha/meta.db"

logging:
  level: info
  format: json
  audit_log: "~/.sangraha/audit.log"
```

See [docs/config-reference.md](docs/config-reference.md) for the full reference.

---

## CLI Reference

```
sangraha
├── init                          # First-time setup
├── server start|stop|status      # Server lifecycle
├── bucket create|delete|list     # Bucket management
├── object put|get|delete|list|cp|mv  # Object operations
├── user create|delete|list|rotate-key
├── config show|set|validate
└── admin gc|export|import
```

---

## Development

### Prerequisites

- Go 1.22+
- Node 20+ (for web dashboard development)
- `make`
- `openssl` (for dev cert generation)

### Build

```bash
go mod download
make build          # current platform
make build-all      # all platforms
make web            # web dashboard only
```

### Test

```bash
make test           # unit tests
make test-race      # unit tests with race detector
make test-cover     # unit tests with HTML coverage report
./scripts/integration-test.sh  # integration tests (requires built binary)
```

### Lint

```bash
make lint           # golangci-lint
make vet            # go vet
make fmt            # gofmt / goimports
```

---

## Architecture

See [CLAUDE.md](CLAUDE.md) for the full architecture blueprint, codebase structure, and contributor conventions.

See [PLAN.md](PLAN.md) for the phase-wise development roadmap.

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).
