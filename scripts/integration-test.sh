#!/usr/bin/env bash
# integration-test.sh — Run integration tests against a live sangraha binary.
#
# This script is a placeholder during Phase 0.
# Full implementation lands in Sprint 1.6.
#
# Usage:
#   ./scripts/integration-test.sh [binary-path]
#
# Environment variables:
#   SANGRAHA_BIN   Path to the sangraha binary (default: ./bin/sangraha)
#   S3_PORT        S3 API port for tests      (default: 19000)
#   ADMIN_PORT     Admin API port for tests   (default: 19001)

set -euo pipefail

SANGRAHA_BIN="${SANGRAHA_BIN:-./bin/sangraha}"
S3_PORT="${S3_PORT:-19000}"
ADMIN_PORT="${ADMIN_PORT:-19001}"

if [ ! -f "${SANGRAHA_BIN}" ]; then
  echo "Binary not found at ${SANGRAHA_BIN} — skipping integration tests."
  echo "(Run 'make build' first, or set SANGRAHA_BIN.)"
  exit 0
fi

echo "sangraha integration tests — placeholder (Sprint 1.6 will populate this)"
echo "Binary : ${SANGRAHA_BIN}"
echo "Ports  : S3=${S3_PORT}  Admin=${ADMIN_PORT}"
echo ""
echo "PASS (no tests yet)"
exit 0
