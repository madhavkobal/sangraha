#!/usr/bin/env bash
# integration-test.sh — Run integration tests against a live sangraha binary.
#
# Usage:
#   ./scripts/integration-test.sh [binary-path]
#
# Environment variables:
#   SANGRAHA_BIN         Path to the sangraha binary (default: ./bin/sangraha)
#   S3_PORT              S3 API port for tests      (default: 19000)
#   ADMIN_PORT           Admin API port for tests   (default: 19001)
#   SANGRAHA_ACCESS_KEY  Root access key for tests  (default: integ-root)
#   SANGRAHA_SECRET_KEY  Root secret key for tests  (default: integ-secret-key-1)

set -euo pipefail

SANGRAHA_BIN="${SANGRAHA_BIN:-./bin/sangraha}"
S3_PORT="${S3_PORT:-19000}"
ADMIN_PORT="${ADMIN_PORT:-19001}"
ROOT_AK="${SANGRAHA_ACCESS_KEY:-integ-root}"
ROOT_SK="${SANGRAHA_SECRET_KEY:-integ-secret-key-1}"

if [ ! -f "${SANGRAHA_BIN}" ]; then
  echo "Binary not found at ${SANGRAHA_BIN} — skipping integration tests."
  echo "(Run 'make build' first, or set SANGRAHA_BIN.)"
  exit 0
fi

# Create a temporary working directory for data and config.
TMPDIR="$(mktemp -d)"
trap 'kill "${SERVER_PID:-}" 2>/dev/null; rm -rf "${TMPDIR}"' EXIT

DATA_DIR="${TMPDIR}/data"
META_PATH="${TMPDIR}/meta.db"
CONFIG_PATH="${TMPDIR}/config.yaml"
mkdir -p "${DATA_DIR}"

cat > "${CONFIG_PATH}" <<EOF
server:
  s3_address: ":${S3_PORT}"
  admin_address: ":${ADMIN_PORT}"
  tls:
    enabled: false
storage:
  backend: localfs
  data_dir: "${DATA_DIR}"
metadata:
  path: "${META_PATH}"
auth:
  root_access_key: "${ROOT_AK}"
logging:
  level: error
  format: text
EOF

echo "Starting sangraha server..."
SANGRAHA_ROOT_SECRET_KEY="${ROOT_SK}" \
  "${SANGRAHA_BIN}" server start --config "${CONFIG_PATH}" &
SERVER_PID=$!

# Wait for the admin health endpoint to be ready (max 15 seconds).
echo "Waiting for server to become ready..."
READY=0
for i in $(seq 1 75); do
  if curl -sf "http://localhost:${ADMIN_PORT}/admin/v1/health" >/dev/null 2>&1; then
    READY=1
    break
  fi
  sleep 0.2
done

if [ "${READY}" -eq 0 ]; then
  echo "Server did not become ready within 15 seconds." >&2
  exit 1
fi
echo "Server is ready."

# Run the Go integration tests.
export SANGRAHA_INTEGRATION=1
export S3_ENDPOINT="http://localhost:${S3_PORT}"
export ADMIN_ENDPOINT="http://localhost:${ADMIN_PORT}"
export SANGRAHA_ACCESS_KEY="${ROOT_AK}"
export SANGRAHA_SECRET_KEY="${ROOT_SK}"

echo "Running integration tests..."
go test -v -timeout 60s ./test/integration/...

echo ""
echo "Integration tests PASSED."
