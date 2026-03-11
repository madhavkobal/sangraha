# sangraha Operations Guide

> Version: 1.0
> Applies to: sangraha v1.x

This guide covers day-to-day operations: deployment, backup, restore, upgrade, log rotation, and TLS certificate renewal.

---

## Table of Contents

1. [Deployment](#1-deployment)
2. [Configuration Reference](#2-configuration-reference)
3. [Starting and Stopping the Server](#3-starting-and-stopping-the-server)
4. [Health Checks and Monitoring](#4-health-checks-and-monitoring)
5. [Backup and Restore](#5-backup-and-restore)
6. [Upgrade Procedure](#6-upgrade-procedure)
7. [Log Rotation](#7-log-rotation)
8. [TLS Certificate Renewal](#8-tls-certificate-renewal)
9. [Garbage Collection](#9-garbage-collection)
10. [Troubleshooting](#10-troubleshooting)

---

## 1. Deployment

### Single-Node (Recommended for ≤ 50 TB)

```bash
# Create data and metadata directories
mkdir -p /var/lib/sangraha/data
mkdir -p /etc/sangraha

# Write config
cat > /etc/sangraha/config.yaml <<EOF
server:
  s3_address: ":9000"
  admin_address: ":9001"
  tls:
    enabled: true
    auto_self_signed: true

storage:
  backend: localfs
  data_dir: /var/lib/sangraha/data

metadata:
  path: /var/lib/sangraha/meta.db

logging:
  level: info
  format: json
  audit_log: /var/log/sangraha/audit.log

limits:
  max_object_size: "5TB"
  max_bucket_count: 1000
  rate_limit_rps: 1000
EOF

# Set root credentials (never store secret in config file)
export SANGRAHA_ROOT_ACCESS_KEY=admin
export SANGRAHA_ROOT_SECRET_KEY=<strong-random-secret>

# Start
./sangraha server start --config /etc/sangraha/config.yaml
```

### systemd Unit

```ini
# /etc/systemd/system/sangraha.service
[Unit]
Description=sangraha object storage
After=network.target

[Service]
Type=simple
User=sangraha
Group=sangraha
EnvironmentFile=/etc/sangraha/env
ExecStart=/usr/local/bin/sangraha server start --config /etc/sangraha/config.yaml
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

```bash
# /etc/sangraha/env  (mode 0600, owned by root)
SANGRAHA_ROOT_ACCESS_KEY=admin
SANGRAHA_ROOT_SECRET_KEY=<strong-random-secret>
```

```bash
systemctl daemon-reload
systemctl enable --now sangraha
```

### Docker

```bash
docker run -d \
  --name sangraha \
  -p 9000:9000 -p 9001:9001 \
  -v /var/lib/sangraha:/data \
  -e SANGRAHA_ROOT_ACCESS_KEY=admin \
  -e SANGRAHA_ROOT_SECRET_KEY=<secret> \
  ghcr.io/madhavkobal/sangraha:1.0.0 \
  server start
```

---

## 2. Configuration Reference

See [`docs/config-reference.md`](config-reference.md) for the full schema. Key runtime-tunable fields (hot-reloadable without restart via `POST /admin/v1/server/reload` or `PUT /admin/v1/config`):

| Field | Default | Hot-reload |
|---|---|---|
| `logging.level` | `info` | ✅ |
| `logging.format` | `json` | ✅ |
| `limits.rate_limit_rps` | `1000` | ✅ |
| `limits.max_bucket_count` | `1000` | ✅ |
| `server.tls.*` | — | ❌ requires restart |
| `storage.backend` | `localfs` | ❌ requires restart |
| `metadata.path` | — | ❌ requires restart |

---

## 3. Starting and Stopping the Server

```bash
# Foreground (development)
sangraha server start --dev

# Background via systemd
systemctl start sangraha
systemctl stop sangraha   # graceful 30s drain

# Status
systemctl status sangraha
sangraha server status    # via admin API
```

Graceful shutdown drains in-flight requests for up to 30 seconds, then forcefully terminates.

---

## 4. Health Checks and Monitoring

### Endpoints

| Endpoint | Auth | Purpose |
|---|---|---|
| `GET /admin/v1/health` | None | Liveness — server is running |
| `GET /admin/v1/ready` | None | Readiness — storage ready |
| `GET /admin/v1/metrics` | None | Prometheus metrics |
| `GET /admin/v1/info` | None | Version and uptime |

### Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: sangraha
    static_configs:
      - targets: ['localhost:9001']
    metrics_path: /admin/v1/metrics
```

### Key Metrics

| Metric | Description |
|---|---|
| `sangraha_requests_total` | Request count by method, path, status |
| `sangraha_request_duration_seconds` | Request latency histogram |
| `sangraha_bytes_transferred_total` | Bytes in/out |
| `sangraha_active_connections` | Current open connections |

### Load Balancer Health Check

Configure your LB to probe `GET /admin/v1/health` every 10s. Remove from rotation on 3 consecutive failures.

---

## 5. Backup and Restore

sangraha data consists of two parts that must be backed up together:

| Component | Location | Contents |
|---|---|---|
| Object data | `storage.data_dir` | Binary object files |
| Metadata DB | `metadata.path` | Bucket/object records, access keys |
| Audit log | `logging.audit_log` | Append-only audit events |

### Online Backup via Admin API

```bash
# Trigger an export (streams a tar.gz)
curl -u admin:secret -X POST https://localhost:9001/admin/v1/export \
  -o backup-$(date +%Y%m%d).tar.gz
```

The export includes both object data and the metadata snapshot, consistent as of the export start time.

### Filesystem Snapshot (Recommended for Large Deployments)

1. Quiesce writes by setting `limits.rate_limit_rps=0` temporarily (all writes return 503).
2. Snapshot the filesystem containing `data_dir` and `metadata.path` using LVM, ZFS, or cloud provider snapshot APIs.
3. Restore `rate_limit_rps` to its operational value.

### Restore Procedure

```bash
# Stop the server
systemctl stop sangraha

# Clear existing data
rm -rf /var/lib/sangraha/data/*
rm -f /var/lib/sangraha/meta.db

# Extract backup
tar -xzf backup-20260308.tar.gz -C /var/lib/sangraha/

# Start the server
systemctl start sangraha
```

### Backup Schedule Recommendations

| Data volume | Frequency | Retention |
|---|---|---|
| < 100 GB | Daily | 30 days |
| 100 GB – 1 TB | Daily incremental, weekly full | 90 days |
| > 1 TB | Hourly incremental, daily full | 180 days |

---

## 6. Upgrade Procedure

sangraha uses semantic versioning. Patch upgrades (`v1.0.x`) are always safe. Minor upgrades (`v1.x.0`) may require config changes — check the migration guide.

```bash
# 1. Download new binary
curl -Lo /tmp/sangraha-new https://github.com/madhavkobal/sangraha/releases/download/v1.1.0/sangraha-linux-amd64
chmod +x /tmp/sangraha-new

# 2. Verify checksum
sha256sum -c SHA256SUMS --ignore-missing

# 3. Take a backup (see Section 5)

# 4. Stop the running server
systemctl stop sangraha

# 5. Replace binary
cp /usr/local/bin/sangraha /usr/local/bin/sangraha.bak
mv /tmp/sangraha-new /usr/local/bin/sangraha

# 6. Apply any config migrations described in the release changelog

# 7. Start the server
systemctl start sangraha

# 8. Verify health
curl http://localhost:9001/admin/v1/health
```

If the upgrade fails, restore the backup binary and config:

```bash
systemctl stop sangraha
mv /usr/local/bin/sangraha.bak /usr/local/bin/sangraha
systemctl start sangraha
```

---

## 7. Log Rotation

### logrotate Configuration

```ini
# /etc/logrotate.d/sangraha
/var/log/sangraha/*.log {
    daily
    rotate 90
    compress
    delaycompress
    missingok
    notifempty
    postrotate
        # Send SIGHUP to trigger log file re-open (future: implemented in v1.1)
        systemctl kill -s HUP sangraha 2>/dev/null || true
    endscript
}
```

### Audit Log Considerations

The audit log is append-only. Rotation is safe because sangraha re-opens the file after SIGHUP. Do not delete the audit log file while the server is running without first sending SIGHUP to force a close/reopen cycle.

For long-term audit retention, ship logs to a SIEM (e.g., Splunk, Elastic, OpenSearch) using a log forwarder like Filebeat or Vector.

---

## 8. TLS Certificate Renewal

### Auto Self-Signed (Development / Internal)

sangraha automatically renews self-signed certificates when they expire if `auto_self_signed: true`. You can also trigger renewal manually:

```bash
# Via admin API
curl -X POST https://localhost:9001/admin/v1/tls/renew

# Via web dashboard → Server → TLS Management → "Renew Now"
```

### Custom Certificate (Production)

```bash
# 1. Obtain new certificate (e.g., from Let's Encrypt via certbot)
certbot renew --cert-name sangraha.example.com

# 2. Copy new cert to expected paths
cp /etc/letsencrypt/live/sangraha.example.com/fullchain.pem /etc/sangraha/tls.crt
cp /etc/letsencrypt/live/sangraha.example.com/privkey.pem   /etc/sangraha/tls.key

# 3. Reload TLS (requires restart; hot-reload not yet supported for TLS files)
systemctl restart sangraha
```

Set up a cron job to run `certbot renew` and restart sangraha 30 days before expiry. Monitor certificate expiry via the `days_until_expiry` field in `GET /admin/v1/tls` or the Monitoring page in the web dashboard (turns red when < 30 days).

---

## 9. Garbage Collection

Garbage collection removes orphaned objects — data files that exist on disk but have no corresponding metadata record. This can occur after a crash during a write.

```bash
# Trigger via CLI
sangraha admin gc --confirm

# Trigger via admin API
curl -X POST https://localhost:9001/admin/v1/gc

# Check status
curl https://localhost:9001/admin/v1/gc/status
```

GC runs asynchronously. The status endpoint returns:
```json
{
  "running": true,
  "scanned": 1024,
  "deleted": 3,
  "freed_bytes": 2097152,
  "last_run": "2026-03-08T10:00:00Z"
}
```

**Recommended schedule:** Run GC weekly during off-peak hours, or after any major bulk-delete operation.

---

## 10. Troubleshooting

### Server won't start

1. Check log output: `journalctl -u sangraha -n 100`
2. Verify config file parses: `sangraha config validate --config /etc/sangraha/config.yaml`
3. Verify metadata DB is not locked: another sangraha process may be running (`pgrep sangraha`)
4. Verify data directory permissions: `ls -la /var/lib/sangraha/`

### S3 clients get 403 Forbidden

1. Verify access key and secret key are correct
2. Verify SigV4 region matches — sangraha accepts `us-east-1` by default
3. Verify system clock skew: SigV4 requires clocks within 5 minutes of server time (`date -u` on client and server)
4. Check audit log for the failed request: `jq 'select(.status == 403)' /var/log/sangraha/audit.log`

### High memory usage

1. Check active connections: `GET /admin/v1/connections`
2. Check for large in-progress multipart uploads: these hold metadata in memory
3. Run garbage collection to free orphaned objects
4. Reduce `limits.max_bucket_count` if many empty buckets are open

### Disk full

1. **Do not delete the metadata DB** — this would make all objects unaddressable
2. Identify largest buckets via the web dashboard Overview page
3. Delete objects from the largest buckets or set lifecycle expiration rules
4. Run GC to free space from orphaned objects
5. Expand the underlying filesystem or move `data_dir` to a larger volume

### Audit log is missing entries

The audit log is append-only and written synchronously. If entries are missing:
1. Verify `logging.audit_log` points to a writable path
2. Check disk space — a full disk will cause silent write failures
3. Verify the sangraha process has write permission on the audit log file
