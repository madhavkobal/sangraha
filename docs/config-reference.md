# sangraha Configuration Reference

> Last updated: 2026-03-08
> Applies to: sangraha v0.1.0+

This document describes every configuration key accepted by sangraha, including its type, default value, and corresponding environment variable override.

---

## Configuration File

The default configuration file location is `~/.sangraha/config.yaml`. Override with:

```bash
sangraha server start --config /path/to/config.yaml
# or
export SANGRAHA_CONFIG=/path/to/config.yaml
```

**Format:** YAML (recommended) or TOML.

All configuration keys can be overridden by environment variables using the pattern:
`SANGRAHA_<SECTION>_<KEY>` (uppercase, dots replaced by underscores).

---

## server

Network and TLS settings for the S3 API and admin/web console servers.

### server.s3_address

| Property | Value |
|---|---|
| Type | `string` |
| Default | `":9000"` |
| Env var | `SANGRAHA_SERVER_S3_ADDRESS` |

The listen address for the S3-compatible API server. Use `:9000` to listen on all interfaces, or `127.0.0.1:9000` to restrict to localhost.

### server.admin_address

| Property | Value |
|---|---|
| Type | `string` |
| Default | `":9001"` |
| Env var | `SANGRAHA_SERVER_ADMIN_ADDRESS` |

The listen address for the admin REST API and embedded web dashboard.

### server.tls.enabled

| Property | Value |
|---|---|
| Type | `bool` |
| Default | `true` |
| Env var | `SANGRAHA_SERVER_TLS_ENABLED` |

Enable TLS for the S3 API server. When `true`, the server uses the certificate at `tls.cert_file` or auto-generates a self-signed certificate if `tls.auto_self_signed` is `true`.

The admin server always runs over plain HTTP on its own port.

### server.tls.cert_file

| Property | Value |
|---|---|
| Type | `string` |
| Default | `""` |
| Env var | `SANGRAHA_SERVER_TLS_CERT_FILE` |

Path to the PEM-encoded TLS certificate file. Required when `tls.enabled: true` and `tls.auto_self_signed: false`.

### server.tls.key_file

| Property | Value |
|---|---|
| Type | `string` |
| Default | `""` |
| Env var | `SANGRAHA_SERVER_TLS_KEY_FILE` |

Path to the PEM-encoded TLS private key file.

### server.tls.auto_self_signed

| Property | Value |
|---|---|
| Type | `bool` |
| Default | `true` |
| Env var | `SANGRAHA_SERVER_TLS_AUTO_SELF_SIGNED` |

When `true` and `tls.enabled: true`, sangraha auto-generates a self-signed TLS certificate at startup if `cert_file` and `key_file` are absent or empty. Suitable for development and internal use; not for public-facing deployments.

---

## storage

Controls the object storage backend.

### storage.backend

| Property | Value |
|---|---|
| Type | `string` |
| Default | `"localfs"` |
| Env var | `SANGRAHA_STORAGE_BACKEND` |
| Allowed values | `localfs`, `badger` |

The storage backend to use for object data:

- **`localfs`** (default): Objects are stored as files on the local filesystem under `storage.data_dir`. Each bucket is a subdirectory; each object key is a file path within that subdirectory. Suitable for all sizes up to disk capacity.
- **`badger`**: Objects are stored in an embedded BadgerDB key-value store. Optimised for small-object workloads and embedded deployments. Recommended maximum: ~100 GB before memory pressure increases.

### storage.data_dir

| Property | Value |
|---|---|
| Type | `string` |
| Default | `"~/.sangraha/data"` |
| Env var | `SANGRAHA_STORAGE_DATA_DIR` |

The root directory for object data. For `localfs`, objects are stored as files under this path. Must be writable by the sangraha process.

---

## metadata

Controls the embedded metadata store.

### metadata.path

| Property | Value |
|---|---|
| Type | `string` |
| Default | `"~/.sangraha/meta.db"` |
| Env var | `SANGRAHA_METADATA_PATH` |

Path to the bbolt (BoltDB) database file that stores all bucket and object metadata, access keys, multipart upload state, and bucket configuration. The directory must exist and be writable. A single-file, embedded database — no external process required.

---

## auth

Authentication and authorisation settings.

### auth.root_access_key

| Property | Value |
|---|---|
| Type | `string` |
| Default | `"root"` |
| Env var | `SANGRAHA_AUTH_ROOT_ACCESS_KEY` |

The access key ID for the root (administrator) user. This key has full access to all API operations and admin endpoints.

> **Note:** The corresponding secret key must be provided **only** via the environment variable `SANGRAHA_ROOT_SECRET_KEY` — never stored in the config file.

### SANGRAHA_ROOT_SECRET_KEY _(environment variable only)_

| Property | Value |
|---|---|
| Type | `string` (env var only) |
| Default | — (no default; must be set) |
| Env var | `SANGRAHA_ROOT_SECRET_KEY` |

The plaintext secret key for the root access key. Set this at startup:

```bash
export SANGRAHA_ROOT_SECRET_KEY=my-secret-key-at-least-20-chars
sangraha server start
```

If not set, the root access key is not provisioned in the metadata store and all API requests will be rejected. The secret must be at least 20 characters.

---

## logging

Controls log output and audit logging.

### logging.level

| Property | Value |
|---|---|
| Type | `string` |
| Default | `"info"` |
| Env var | `SANGRAHA_LOGGING_LEVEL` |
| Allowed values | `debug`, `info`, `warn`, `error` |

Minimum log severity level to emit. Use `debug` for detailed request tracing during development; use `warn` or `error` in production to reduce log volume.

### logging.format

| Property | Value |
|---|---|
| Type | `string` |
| Default | `"json"` |
| Env var | `SANGRAHA_LOGGING_FORMAT` |
| Allowed values | `json`, `text` |

Log output format:
- **`json`**: Structured JSON lines, suitable for log aggregation systems (Elasticsearch, Loki, Datadog).
- **`text`**: Human-readable coloured output, suitable for interactive terminal use.

`text` format is also automatically selected when `server start --dev` flag is used.

### logging.audit_log

| Property | Value |
|---|---|
| Type | `string` |
| Default | `""` (disabled) |
| Env var | `SANGRAHA_LOGGING_AUDIT_LOG` |

Path to the append-only audit log file. Every API request — including the authenticated identity, operation, resource, source IP, HTTP status, bytes transferred, and duration — is written here in JSON format.

When empty, audit events are written to the main log stream at `info` level but not to a separate file.

```bash
logging:
  audit_log: "/var/log/sangraha/audit.log"
```

The audit log is append-only. Deletion requires explicit operator action.

---

## limits

Server-wide resource limits and timeouts.

### limits.max_object_size

| Property | Value |
|---|---|
| Type | `string` (size with unit) |
| Default | `"5TB"` |
| Env var | `SANGRAHA_LIMITS_MAX_OBJECT_SIZE` |

Maximum allowed size for a single object PUT (not multipart). Accepts human-readable units: `KB`, `MB`, `GB`, `TB`.

### limits.max_bucket_count

| Property | Value |
|---|---|
| Type | `int` |
| Default | `1000` |
| Env var | `SANGRAHA_LIMITS_MAX_BUCKET_COUNT` |

Maximum number of buckets allowed per server instance. Set to `0` for unlimited.

### limits.rate_limit_rps

| Property | Value |
|---|---|
| Type | `int` |
| Default | `1000` |
| Env var | `SANGRAHA_LIMITS_RATE_LIMIT_RPS` |

Maximum requests per second allowed per access key (token bucket). Requests exceeding this limit receive `HTTP 429 Too Many Requests`. Set to `0` to disable rate limiting.

### limits.read_timeout

| Property | Value |
|---|---|
| Type | `duration` |
| Default | `30s` |
| Env var | `SANGRAHA_LIMITS_READ_TIMEOUT` |

Maximum duration for reading the full request (including body). Connections that do not complete reading within this time are closed.

### limits.write_timeout

| Property | Value |
|---|---|
| Type | `duration` |
| Default | `30s` |
| Env var | `SANGRAHA_LIMITS_WRITE_TIMEOUT` |

Maximum duration for writing the full response. Relevant for large `GetObject` responses where slow clients may stall the connection.

### limits.idle_timeout

| Property | Value |
|---|---|
| Type | `duration` |
| Default | `120s` |
| Env var | `SANGRAHA_LIMITS_IDLE_TIMEOUT` |

Maximum time to wait for the next request on a keep-alive connection before closing it.

---

## Full Example Config

```yaml
server:
  s3_address: ":9000"
  admin_address: ":9001"
  tls:
    enabled: true
    cert_file: "/etc/sangraha/tls.crt"
    key_file: "/etc/sangraha/tls.key"
    auto_self_signed: true   # generate if cert/key absent

storage:
  backend: localfs
  data_dir: "/var/lib/sangraha/data"

metadata:
  path: "/var/lib/sangraha/meta.db"

auth:
  root_access_key: "root"
  # root secret key: set SANGRAHA_ROOT_SECRET_KEY env var only

logging:
  level: info
  format: json
  audit_log: "/var/log/sangraha/audit.log"

limits:
  max_object_size: "5TB"
  max_bucket_count: 1000
  rate_limit_rps: 1000
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
```

---

## Environment Variable Quick Reference

| Environment Variable | Config Key | Description |
|---|---|---|
| `SANGRAHA_CONFIG` | — | Path to config file |
| `SANGRAHA_SERVER_S3_ADDRESS` | `server.s3_address` | S3 API listen address |
| `SANGRAHA_SERVER_ADMIN_ADDRESS` | `server.admin_address` | Admin API listen address |
| `SANGRAHA_SERVER_TLS_ENABLED` | `server.tls.enabled` | Enable TLS |
| `SANGRAHA_SERVER_TLS_CERT_FILE` | `server.tls.cert_file` | TLS certificate path |
| `SANGRAHA_SERVER_TLS_KEY_FILE` | `server.tls.key_file` | TLS key path |
| `SANGRAHA_SERVER_TLS_AUTO_SELF_SIGNED` | `server.tls.auto_self_signed` | Auto-generate self-signed cert |
| `SANGRAHA_STORAGE_BACKEND` | `storage.backend` | Storage backend (`localfs`/`badger`) |
| `SANGRAHA_STORAGE_DATA_DIR` | `storage.data_dir` | Object data root directory |
| `SANGRAHA_METADATA_PATH` | `metadata.path` | Metadata database path |
| `SANGRAHA_AUTH_ROOT_ACCESS_KEY` | `auth.root_access_key` | Root access key ID |
| `SANGRAHA_ROOT_SECRET_KEY` | _(env only)_ | Root secret key (never in config file) |
| `SANGRAHA_LOGGING_LEVEL` | `logging.level` | Log level |
| `SANGRAHA_LOGGING_FORMAT` | `logging.format` | Log format (`json`/`text`) |
| `SANGRAHA_LOGGING_AUDIT_LOG` | `logging.audit_log` | Audit log file path |
| `SANGRAHA_LIMITS_MAX_OBJECT_SIZE` | `limits.max_object_size` | Max single-object size |
| `SANGRAHA_LIMITS_MAX_BUCKET_COUNT` | `limits.max_bucket_count` | Max bucket count |
| `SANGRAHA_LIMITS_RATE_LIMIT_RPS` | `limits.rate_limit_rps` | Rate limit (req/s per key) |
| `SANGRAHA_LIMITS_READ_TIMEOUT` | `limits.read_timeout` | HTTP read timeout |
| `SANGRAHA_LIMITS_WRITE_TIMEOUT` | `limits.write_timeout` | HTTP write timeout |
| `SANGRAHA_LIMITS_IDLE_TIMEOUT` | `limits.idle_timeout` | HTTP idle timeout |
| `SANGRAHA_ACCESS_KEY` | — | CLI: access key for API calls |
| `SANGRAHA_SECRET_KEY` | — | CLI: secret key for API calls |
| `SANGRAHA_SERVER` | — | CLI: S3 API server URL |
| `SANGRAHA_ADMIN_URL` | — | CLI: admin API server URL |
