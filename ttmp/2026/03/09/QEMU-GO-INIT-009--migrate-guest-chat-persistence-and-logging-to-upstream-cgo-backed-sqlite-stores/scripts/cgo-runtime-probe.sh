#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../../../.." && pwd)
cd "${ROOT_DIR}"

mkdir -p build

OUT=${1:-build/init-cgo-probe}

echo "==> building CGO guest probe: ${OUT}"
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -o "${OUT}" ./cmd/init

echo
echo "==> file ${OUT}"
file "${OUT}"

echo
echo "==> ldd ${OUT}"
ldd "${OUT}"
