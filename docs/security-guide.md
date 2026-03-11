# sangraha Security Guide

> Version: 1.0
> Applies to: sangraha v1.x

This guide covers the security model, key rotation, access control strategy, audit log management, and TLS hardening.

---

## Table of Contents

1. [Security Model Overview](#1-security-model-overview)
2. [Authentication](#2-authentication)
3. [Access Key Rotation](#3-access-key-rotation)
4. [Authorization: ACLs and Bucket Policies](#4-authorization-acls-and-bucket-policies)
5. [Transport Security (TLS)](#5-transport-security-tls)
6. [Server-Side Encryption](#6-server-side-encryption)
7. [Presigned URLs](#7-presigned-urls)
8. [Audit Log Management](#8-audit-log-management)
9. [Security Hardening Checklist](#9-security-hardening-checklist)
10. [Incident Response](#10-incident-response)

---

## 1. Security Model Overview

sangraha applies a layered security model:

```
Request
  │
  ├─ TLS 1.2+ (transport encryption)
  │
  ├─ SigV4 authentication (identity verification)
  │
  ├─ Rate limiting (per-key token bucket)
  │
  ├─ IAM policy evaluation (action authorisation)
  │
  ├─ Canned ACL check (bucket/object permissions)
  │
  └─ Object data (optionally AES-256-GCM encrypted at rest)
```

**Default-deny:** Every request is denied unless an explicit allow is found. There is no guest or anonymous access by default (unless a bucket ACL explicitly enables `public-read`).

---

## 2. Authentication

### S3 API — AWS Signature Version 4

All S3 API requests must be signed with AWS Signature Version 4 (SigV4). SigV4 signs the request URL, headers, and body with HMAC-SHA256 using the access key's signing key. This prevents:
- Credential theft (the secret key is never transmitted)
- Replay attacks (requests include a timestamp; sangraha rejects requests older than 5 minutes)
- Request tampering (the signature covers headers and body hash)

### Admin API — Bearer Token

The admin API (port 9001) uses a Bearer token derived from the root access key. The token is issued by presenting the access key + secret key to the login flow in the web dashboard.

### Root Credentials

The root access key and secret key grant full control over the cluster. Protect them:
- Never store the root secret key in the config file — use the `SANGRAHA_ROOT_SECRET_KEY` environment variable
- Store the env var in a file with mode `0600` owned by the sangraha service user
- Rotate the root secret key quarterly (see Section 3)

### Secret Storage

All secret keys are stored as **bcrypt hashes** (cost ≥ 12) in the metadata database. The raw secret is never retrievable after creation — if lost, rotate the key.

The signing key (used for SigV4 HMAC operations) is stored in plaintext in the metadata DB. This is required for SigV4 verification. **Phase 3 will encrypt the signing key at rest using an operator-supplied master key.**

---

## 3. Access Key Rotation

Rotate access keys regularly. The web dashboard (Users → user → Access Keys → Rotate) or CLI:

```bash
# Rotate key via CLI
sangraha user rotate-key <access-key> --confirm

# Via admin API
curl -X POST https://localhost:9001/admin/v1/users/<access-key>/keys/rotate \
  -H "Authorization: Bearer <token>"
```

Rotation atomically deletes the old key and creates a new one. The new secret key is shown **once** — save it immediately. The old key stops working immediately after rotation.

### Rotation Schedule Recommendations

| Key type | Recommended rotation interval |
|---|---|
| Root access key | Quarterly |
| CI/CD service account | Monthly |
| Developer credentials | Every 6 months or on personnel change |
| Shared team key | Immediately on team member departure |

---

## 4. Authorization: ACLs and Bucket Policies

### Canned ACLs

Set with the `x-amz-acl` header on `CreateBucket` or `PutObject`:

| ACL | Effect |
|---|---|
| `private` (default) | Owner only; no public access |
| `public-read` | Any unauthenticated request can `GetObject` |
| `public-read-write` | Any unauthenticated request can `PutObject` and `GetObject` |
| `authenticated-read` | Any authenticated user (valid SigV4) can read |

**Warning:** `public-read-write` allows anonymous writes. Only use for controlled test environments.

### Bucket Policies

Bucket policies use a subset of the AWS IAM policy language. Example — allow a specific user read-only access:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": "arn:aws:iam::000000000000:user/alice"},
      "Action": ["s3:GetObject", "s3:ListBucket"],
      "Resource": [
        "arn:aws:s3:::my-bucket",
        "arn:aws:s3:::my-bucket/*"
      ]
    }
  ]
}
```

Apply with:
```bash
sangraha bucket policy set my-bucket policy.json
```

### Evaluation Order

1. Explicit **Deny** in any policy → request denied
2. Explicit **Allow** in bucket policy → request allowed
3. Canned ACL check → allow or deny
4. Default → **Deny**

---

## 5. Transport Security (TLS)

### Minimum Requirements

sangraha enforces TLS 1.2 as the minimum; TLS 1.3 is preferred. The following cipher suites are explicitly disabled:
- RC4 (all variants)
- 3DES
- Export-grade ciphers
- Weak DHE groups (< 2048 bits)

### Certificate Configuration

```yaml
server:
  tls:
    enabled: true
    cert_file: /etc/sangraha/tls.crt
    key_file:  /etc/sangraha/tls.key
    auto_self_signed: false  # set true only for development
```

### Hardening Recommendations

1. Use a certificate from a trusted CA (Let's Encrypt, your internal PKI, or a commercial CA) in production — not the auto-generated self-signed cert
2. Set `auto_self_signed: false` in production
3. Configure HSTS if exposing sangraha to the internet (requires a reverse proxy)
4. Prefer ECDSA P-256 certificates over RSA 2048 for better performance
5. Disable HTTP (port 80) if running behind a reverse proxy; sangraha has no HTTP-to-HTTPS redirect by default

### Verifying TLS Configuration

```bash
# Check negotiated TLS version and cipher
openssl s_client -connect localhost:9000 -tls1_2 2>&1 | grep -E "Protocol|Cipher"

# Check certificate expiry
curl -I https://localhost:9000/admin/v1/health --insecure 2>&1 | grep -i expire

# Admin API certificate info
curl https://localhost:9001/admin/v1/tls
```

---

## 6. Server-Side Encryption

Enable AES-256-GCM encryption for objects at rest on a per-bucket basis:

```bash
# Enable SSE on a bucket via S3 API
aws s3api put-bucket-encryption \
  --bucket my-bucket \
  --server-side-encryption-configuration \
    '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'
```

Or include `x-amz-server-side-encryption: AES256` on individual `PutObject` requests.

### How It Works

1. A random 256-bit data encryption key (DEK) is generated per object
2. Object data is encrypted with AES-256-GCM using the DEK
3. The DEK is encrypted with a bucket-level key encryption key (KEK) derived from the server's master key using HKDF-SHA256
4. The encrypted DEK is stored in the object's metadata record alongside the GCM nonce

**Phase 3** will support external KMS (AWS KMS, HashiCorp Vault) for KEK management.

### Limitations

- Encryption is applied at write time; existing unencrypted objects are not retroactively encrypted
- The master key is derived from `SANGRAHA_ROOT_SECRET_KEY` — changing the root secret key requires re-encrypting all DEKs (a future migration utility)
- Range requests on encrypted objects are supported; sangraha decrypts the full object and serves the requested range

---

## 7. Presigned URLs

Presigned URLs grant time-limited access to a specific object without requiring AWS credentials in the client.

```bash
# Generate via admin API (max expiry: 7 days)
curl -X POST https://localhost:9001/admin/v1/presign \
  -H "Authorization: Bearer <token>" \
  -d '{"bucket":"my-bucket","key":"report.pdf","method":"GET","expires_in":3600}'
```

Response:
```json
{
  "url": "https://localhost:9000/my-bucket/report.pdf?X-Amz-Expires=3600&...",
  "expires_at": "2026-03-08T11:00:00Z"
}
```

### Security Considerations

- The maximum expiry is 7 days (matching the AWS S3 maximum)
- A presigned URL cannot be revoked before expiry — do not generate presigned URLs with long expiries for sensitive objects
- If an access key is rotated, all presigned URLs signed with the old key immediately become invalid
- Presigned URLs are logged in the audit log at generation time and at access time

---

## 8. Audit Log Management

### Log Format

Each event is a JSON line:
```json
{
  "time": "2026-03-08T10:00:00Z",
  "request_id": "01HXYZ123...",
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

### Protecting the Audit Log

- The audit log file is append-only — do not grant write or delete permissions to service accounts
- The sangraha process writes audit events synchronously before returning the HTTP response
- Rotate the log with `logrotate` (see Operations Guide Section 7)
- Ship logs to an immutable SIEM or cloud storage for long-term retention

### Querying the Audit Log

```bash
# Last 100 events for user alice
curl "https://localhost:9001/admin/v1/audit?user=alice&limit=100"

# Events in a time range
curl "https://localhost:9001/admin/v1/audit?from=2026-03-01T00:00:00Z&to=2026-03-08T23:59:59Z"

# Failed requests only (filter by status in post-processing)
curl "https://localhost:9001/admin/v1/audit?limit=1000" | jq '[.[] | select(.status >= 400)]'

# Export to file for forensics
jq . /var/log/sangraha/audit.log > audit-export.json
```

### Retention Policy

| Compliance framework | Minimum retention |
|---|---|
| PCI-DSS | 12 months |
| SOC 2 | 12 months |
| HIPAA | 6 years |
| GDPR | As long as data is processed |

Set up logrotate to compress and retain audit logs for the required duration. For regulated industries, configure shipping to WORM (write-once read-many) storage.

---

## 9. Security Hardening Checklist

Use this checklist before going to production:

### Authentication & Keys

- [ ] Root secret key set via environment variable, not config file
- [ ] Config file at mode `0600`; env file at mode `0600`
- [ ] Root access key rotated from the default
- [ ] Individual user accounts created for each application and team member
- [ ] No shared credentials across services

### Network & TLS

- [ ] TLS enabled (`server.tls.enabled: true`)
- [ ] `auto_self_signed: false` in production (CA-signed cert in use)
- [ ] Ports 9000 and 9001 not exposed to the internet without a reverse proxy and WAF
- [ ] Reverse proxy configured to forward client IP in `X-Forwarded-For`

### Authorization

- [ ] All buckets have explicit ACL — no accidental `public-read-write`
- [ ] Bucket policies reviewed and match least-privilege principle
- [ ] Regular user accounts do NOT have admin API access

### Encryption

- [ ] SSE enabled on all buckets containing sensitive data
- [ ] Presigned URL expiry ≤ 1 hour for sensitive objects

### Audit & Monitoring

- [ ] Audit log path configured and writable
- [ ] Audit log shipped to immutable storage or SIEM
- [ ] Prometheus metrics scrape configured with alert on error rate > 1%
- [ ] TLS expiry alert configured (< 30 days)
- [ ] GC scheduled weekly

### Updates

- [ ] Alerts configured for new sangraha releases (GitHub Releases RSS)
- [ ] Security advisories monitored (`github.com/madhavkobal/sangraha/security/advisories`)

---

## 10. Incident Response

### Suspected Credential Compromise

```bash
# 1. Immediately rotate the compromised key
sangraha user rotate-key <access-key> --confirm

# 2. Query audit log for activity with the compromised key
curl "https://localhost:9001/admin/v1/audit?user=<access-key>&limit=1000" > incident-audit.json

# 3. Identify accessed buckets and objects
jq '[.[] | {time, action, bucket, key, source_ip}]' incident-audit.json

# 4. If root key was compromised: take server offline, restore from backup
systemctl stop sangraha

# 5. Notify affected users of any data exposure
```

### Suspected Unauthorised Data Access

1. Review audit log for all `s3:GetObject` events in the suspect time window
2. Check source IPs against known-good ranges
3. If public ACL was inadvertently set, change it immediately:
   ```bash
   sangraha bucket create <name> --acl private  # recreate with correct ACL
   ```
4. If data was exfiltrated: notify affected parties per your data breach policy

### Server Compromise

1. Isolate the server (network-level block)
2. Do not attempt forensics on the running system — take a disk image first
3. Restore from the last known-good backup (see Operations Guide Section 5)
4. Rotate **all** access keys after restore
5. Review audit logs from a read-only copy for signs of persistence
