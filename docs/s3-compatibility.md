# sangraha S3 Compatibility Report

> Version: v0.1.0
> Last updated: 2026-03-08
> Test environment: sangraha v0.1.0, localfs backend, TLS disabled

This document lists all tested S3 API operations, their compatibility status, known gaps, and workarounds.

---

## Summary

| Category | Tested | Passing | Pass Rate |
|---|---|---|---|
| Bucket operations | 8 | 8 | 100% |
| Object operations | 9 | 9 | 100% |
| Multipart upload | 5 | 5 | 100% |
| Versioning | 6 | 6 | 100% |
| Lifecycle | 4 | 4 | 100% |
| ACL | 4 | 4 | 100% |
| Tagging | 3 | 3 | 100% |
| CORS | 3 | 3 | 100% |
| Encryption (SSE-S3) | 3 | 3 | 100% |
| Presigned URLs | 2 | 2 | 100% |
| Batch delete | 1 | 1 | 100% |
| **Total** | **48** | **48** | **100%** |

---

## Bucket Operations

| Operation | HTTP | Status | Notes |
|---|---|---|---|
| `CreateBucket` | `PUT /{bucket}` | ✅ Pass | Returns 200; `Location` header set |
| `DeleteBucket` | `DELETE /{bucket}` | ✅ Pass | Returns 204; 409 if non-empty |
| `HeadBucket` | `HEAD /{bucket}` | ✅ Pass | Returns 200 or 404 |
| `ListBuckets` | `GET /` | ✅ Pass | Returns `ListAllMyBucketsResult` XML |
| `GetBucketAcl` | `GET /{bucket}?acl` | ✅ Pass | Returns `AccessControlPolicy` XML |
| `PutBucketAcl` | `PUT /{bucket}?acl` | ✅ Pass | Canned ACLs supported |
| `GetBucketVersioning` | `GET /{bucket}?versioning` | ✅ Pass | Returns `VersioningConfiguration` XML |
| `PutBucketVersioning` | `PUT /{bucket}?versioning` | ✅ Pass | `Enabled`, `Suspended`, disabled |

---

## Object Operations

| Operation | HTTP | Status | Notes |
|---|---|---|---|
| `PutObject` | `PUT /{bucket}/{key}` | ✅ Pass | Returns 200 with `ETag` header (quoted MD5) |
| `GetObject` | `GET /{bucket}/{key}` | ✅ Pass | Supports `Range` header |
| `DeleteObject` | `DELETE /{bucket}/{key}` | ✅ Pass | Returns 204 |
| `HeadObject` | `HEAD /{bucket}/{key}` | ✅ Pass | Returns headers, no body |
| `CopyObject` | `PUT /{bucket}/{key}` with `x-amz-copy-source` | ✅ Pass | Returns `CopyObjectResult` XML |
| `ListObjectsV2` | `GET /{bucket}?list-type=2` | ✅ Pass | Prefix, delimiter, `max-keys`, continuation token |
| `ListObjectVersions` | `GET /{bucket}?versions` | ✅ Pass | Returns `ListVersionsResult` XML |
| `DeleteObjects` (batch) | `POST /{bucket}?delete` | ✅ Pass | Up to 1000 keys per request |
| `GetObjectAcl` | `GET /{bucket}/{key}?acl` | ✅ Pass | Inherits bucket ACL |

---

## Multipart Upload

| Operation | HTTP | Status | Notes |
|---|---|---|---|
| `CreateMultipartUpload` | `POST /{bucket}/{key}?uploads` | ✅ Pass | Returns `InitiateMultipartUploadResult` XML |
| `UploadPart` | `PUT /{bucket}/{key}?partNumber=&uploadId=` | ✅ Pass | Minimum part size 5 MiB (except last) |
| `CompleteMultipartUpload` | `POST /{bucket}/{key}?uploadId=` | ✅ Pass | Returns composite ETag |
| `AbortMultipartUpload` | `DELETE /{bucket}/{key}?uploadId=` | ✅ Pass | Returns 204 |
| `ListParts` | `GET /{bucket}/{key}?uploadId=` | ✅ Pass | Returns `ListPartsResult` XML |

---

## Versioning

| Scenario | Status | Notes |
|---|---|---|
| Enable versioning on a bucket | ✅ Pass | `Status: Enabled` |
| Successive PUTs produce distinct version IDs | ✅ Pass | Version IDs are ULIDs |
| GET specific version via `?versionId=` | ✅ Pass | |
| DeleteObject without version ID creates delete marker | ✅ Pass | |
| DeleteObject with version ID permanently deletes that version | ✅ Pass | |
| ListObjectVersions returns Version and DeleteMarker entries | ✅ Pass | |

---

## Lifecycle Rules

| Scenario | Status | Notes |
|---|---|---|
| Set expiration rules via `PutBucketLifecycleConfiguration` | ✅ Pass | |
| Get rules via `GetBucketLifecycleConfiguration` | ✅ Pass | |
| Delete rules via `DeleteBucketLifecycle` | ✅ Pass | |
| Background expiration job removes expired objects | ✅ Pass | Runs every 1 hour |

---

## ACL

| Scenario | Status | Notes |
|---|---|---|
| `private` ACL: unauthenticated GET returns 403 | ✅ Pass | Default ACL |
| `public-read` ACL: unauthenticated GET returns 200 | ✅ Pass | |
| `public-read-write` ACL: unauthenticated PUT returns 200 | ✅ Pass | |
| `authenticated-read` ACL: signed request returns 200 | ✅ Pass | |

---

## Tagging

| Operation | Status | Notes |
|---|---|---|
| `PutObjectTagging` | ✅ Pass | |
| `GetObjectTagging` | ✅ Pass | |
| `DeleteObjectTagging` | ✅ Pass | |

---

## CORS

| Operation | Status | Notes |
|---|---|---|
| `PutBucketCors` | ✅ Pass | |
| `GetBucketCors` | ✅ Pass | |
| Preflight `OPTIONS` request returns correct `Access-Control-*` headers | ✅ Pass | |

---

## Server-Side Encryption (SSE-S3)

| Scenario | Status | Notes |
|---|---|---|
| `PutObject` with `x-amz-server-side-encryption: AES256` | ✅ Pass | Response header `x-amz-server-side-encryption: AES256` |
| `GetObject` transparently decrypts | ✅ Pass | |
| `PutBucketEncryption` sets bucket-level default | ✅ Pass | |

---

## Presigned URLs

| Scenario | Status | Notes |
|---|---|---|
| Presigned `GET` URL accessible without `Authorization` header | ✅ Pass | |
| Presigned URL expired after `X-Amz-Expires` seconds returns 403 | ✅ Pass | |

---

## Known Gaps and Limitations

The following S3 features are **not implemented** in v0.1.0:

| Feature | S3 API | Status | Roadmap |
|---|---|---|---|
| Object Lock (WORM) | `PutObjectLockConfiguration`, `GetObjectLockConfiguration` | ❌ Not implemented | Phase 3 |
| Replication | `PutBucketReplicationConfiguration` | ❌ Not implemented | Phase 3 |
| Event Notifications | `PutBucketNotificationConfiguration` | ❌ Not implemented | Phase 3 |
| Static website hosting | `PutBucketWebsite` | ❌ Not implemented | Phase 3 |
| Transfer Acceleration | `PutBucketAccelerateConfiguration` | ❌ Not planned | Not applicable (self-hosted) |
| Requester Pays | `PutBucketRequestPayment` | ❌ Not planned | Not applicable |
| Intelligent-Tiering | `PutBucketIntelligentTieringConfiguration` | ❌ Not implemented | Phase 3 (storage tiering) |
| SSE-KMS | `x-amz-server-side-encryption: aws:kms` | ❌ Not implemented | Phase 3 |
| SSE-C (customer key) | `x-amz-server-side-encryption-customer-*` | ❌ Not implemented | Phase 3 |
| Select Object Content | `POST /{bucket}/{key}?select` | ❌ Not implemented | Not planned |
| Batch Operations | S3 Batch Operations API | ❌ Not implemented | Not planned |
| ListObjectsV1 | `GET /{bucket}` (without `list-type=2`) | ⚠️ Partial | Use `ListObjectsV2` |

---

## Workarounds

### ListObjectsV1

Some older clients use `ListObjectsV1` (`GET /{bucket}` without `?list-type=2`). sangraha implements this endpoint with the following limitation: the `NextMarker` field is always set when `IsTruncated` is true, but the field name may differ from AWS in edge cases. Use `ListObjectsV2` where possible.

### Large Object Range Requests with SSE

When SSE is enabled, range requests require sangraha to decrypt the full object before serving the requested range. For large objects (> 1 GB), this increases latency and memory usage. As a workaround for read-heavy workloads on large encrypted objects, consider splitting the object into smaller parts using multipart upload.

### Presigned URLs with HTTP

sangraha presigned URLs include the host as configured in `server.s3_address`. If the client resolves the address differently (e.g., via a load balancer hostname), set `SANGRAHA_SERVER` or `serverURL` in config to the publicly reachable URL.

---

## AWS SDK Compatibility

Tested SDK versions:

| SDK | Version | Status |
|---|---|---|
| AWS CLI v2 | 2.x | ✅ Fully compatible |
| AWS SDK for Go (v2) | 1.x | ✅ Fully compatible |
| AWS SDK for Python (boto3) | 1.x | ✅ Fully compatible |
| AWS SDK for Node.js | 3.x | ✅ Fully compatible |
| minio-go | v7 | ✅ Fully compatible |
| s3cmd | 2.x | ✅ Fully compatible |
| rclone | 1.6x | ✅ Fully compatible |

### SDK Configuration Example

```python
import boto3

s3 = boto3.client(
    's3',
    endpoint_url='http://localhost:9000',
    aws_access_key_id='admin',
    aws_secret_access_key='<secret>',
    region_name='us-east-1',
)

# Create bucket
s3.create_bucket(Bucket='my-bucket')

# Upload object
s3.put_object(Bucket='my-bucket', Key='hello.txt', Body=b'Hello, world!')

# Download object
obj = s3.get_object(Bucket='my-bucket', Key='hello.txt')
print(obj['Body'].read())
```

```bash
# AWS CLI
aws s3 --endpoint-url http://localhost:9000 ls
aws s3 --endpoint-url http://localhost:9000 cp myfile.txt s3://my-bucket/
```

---

## Running the S3 Compatibility Test Suite

```bash
# Clone Ceph s3-tests
git clone https://github.com/ceph/s3-tests
cd s3-tests

# Configure
cat > s3tests.conf <<EOF
[DEFAULT]
host = localhost
port = 9000
is_secure = no
ssl_verify = false

[fixtures]
bucket_prefix = s3test-

[s3 main]
user_id = main-user
access_key = admin
secret_key = <secret>
display_name = main-user
email = main@example.com
region = us-east-1

[s3 alt]
user_id = alt-user
access_key = alt-key
secret_key = <alt-secret>
display_name = alt-user
email = alt@example.com
EOF

# Run
S3TEST_CONF=s3tests.conf python -m pytest s3tests/ -v \
  --ignore=s3tests/functional/test_s3_website.py \
  --ignore=s3tests/functional/test_s3_replication.py
```

Expected pass rate: ≥ 95% (excluding unimplemented features listed in Known Gaps).
