#!/usr/bin/env bash
# build.sh — Cross-compile sangraha for all target platforms.
# Outputs to ./bin/  Named sangraha-<os>-<arch>[.exe].
# Usage: ./scripts/build.sh [--clean]
set -euo pipefail

BINARY="sangraha"
BIN_DIR="bin"
CMD="./cmd/sangraha"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS="-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

if [[ "${1:-}" == "--clean" ]]; then
  rm -rf "${BIN_DIR}"
fi
mkdir -p "${BIN_DIR}"

for PLATFORM in "${PLATFORMS[@]}"; do
  OS="${PLATFORM%%/*}"
  ARCH="${PLATFORM##*/}"
  EXT=""
  [[ "${OS}" == "windows" ]] && EXT=".exe"
  OUT="${BIN_DIR}/${BINARY}-${OS}-${ARCH}${EXT}"
  echo "Building ${OUT} ..."
  GOOS="${OS}" GOARCH="${ARCH}" go build \
    -ldflags "${LDFLAGS}" \
    -o "${OUT}" \
    "${CMD}"
done

echo ""
echo "Generating SHA256SUMS ..."
(cd "${BIN_DIR}" && sha256sum ${BINARY}-* > SHA256SUMS)
cat "${BIN_DIR}/SHA256SUMS"

echo ""
echo "Build complete. Artifacts in ${BIN_DIR}/"
