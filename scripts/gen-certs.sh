#!/usr/bin/env bash
# gen-certs.sh — Generate a self-signed ECDSA P-256 TLS certificate for development.
#
# Usage:
#   ./scripts/gen-certs.sh [output-dir]
#
# Outputs:
#   <output-dir>/tls.crt  — PEM certificate
#   <output-dir>/tls.key  — PEM private key (ECDSA P-256)
#
# The certificate is valid for 825 days (Apple/browser limit) and covers:
#   localhost, 127.0.0.1, ::1
#
# DO NOT use these certificates in production.

set -euo pipefail

OUTPUT_DIR="${1:-certs}"

command -v openssl >/dev/null 2>&1 || { echo "openssl is required but not installed."; exit 1; }

mkdir -p "${OUTPUT_DIR}"

CERT="${OUTPUT_DIR}/tls.crt"
KEY="${OUTPUT_DIR}/tls.key"
SUBJECT="/CN=sangraha-dev/O=sangraha/C=US"

# Generate ECDSA P-256 private key
openssl ecparam \
  -name prime256v1 \
  -genkey \
  -noout \
  -out "${KEY}"

# Generate self-signed certificate with SANs
openssl req \
  -new \
  -x509 \
  -key "${KEY}" \
  -out "${CERT}" \
  -days 825 \
  -subj "${SUBJECT}" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1,IP:::1" \
  -addext "keyUsage=digitalSignature,keyEncipherment" \
  -addext "extendedKeyUsage=serverAuth"

echo ""
echo "Generated TLS certificate:"
echo "  Certificate : ${CERT}"
echo "  Private key : ${KEY}"
echo ""
openssl x509 -in "${CERT}" -noout -subject -issuer -dates -fingerprint -sha256
echo ""
echo "NOTE: This is a self-signed certificate for development only."
echo "      Configure your S3 client with --no-verify-ssl or trust the cert."
