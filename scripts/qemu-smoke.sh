#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BUILD_DIR=${BUILD_DIR:-"${ROOT_DIR}/build"}
QEMU_BIN=${QEMU_BIN:-qemu-system-x86_64}
KERNEL_IMAGE=${KERNEL_IMAGE:-$(find /boot -maxdepth 1 -name 'vmlinuz-*' -readable 2>/dev/null | sort | tail -n 1)}
HOST_PORT=${HOST_PORT:-18080}
GUEST_PORT=${GUEST_PORT:-8080}
QEMU_MEMORY=${QEMU_MEMORY:-512}
QEMU_APPEND=${QEMU_APPEND:-console=ttyS0 rdinit=/init ip=dhcp}
QEMU_LOG=${QEMU_LOG:-"${BUILD_DIR}/qemu-smoke.log"}

if [[ -z "${KERNEL_IMAGE}" ]]; then
  echo "KERNEL_IMAGE is not set and no readable /boot/vmlinuz-* image was found. Set KERNEL_IMAGE to a readable bzImage/vmlinuz path." >&2
  exit 1
fi

if [[ ! -r "${KERNEL_IMAGE}" ]]; then
  echo "KERNEL_IMAGE=${KERNEL_IMAGE} is not readable by the current user" >&2
  exit 1
fi

if [[ ! -f "${BUILD_DIR}/initramfs.cpio.gz" ]]; then
  echo "missing ${BUILD_DIR}/initramfs.cpio.gz; run 'make initramfs' first" >&2
  exit 1
fi

mkdir -p "${BUILD_DIR}"
rm -f "${QEMU_LOG}"

"${QEMU_BIN}" \
  -m "${QEMU_MEMORY}" \
  -nographic \
  -no-reboot \
  -kernel "${KERNEL_IMAGE}" \
  -initrd "${BUILD_DIR}/initramfs.cpio.gz" \
  -append "${QEMU_APPEND}" \
  -nic "user,model=virtio-net-pci,hostfwd=tcp::${HOST_PORT}-:${GUEST_PORT}" \
  >"${QEMU_LOG}" 2>&1 &
QEMU_PID=$!

cleanup() {
  if kill -0 "${QEMU_PID}" 2>/dev/null; then
    kill "${QEMU_PID}" 2>/dev/null || true
    wait "${QEMU_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

for _ in $(seq 1 80); do
  if curl -fsS "http://127.0.0.1:${HOST_PORT}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done

curl -fsS "http://127.0.0.1:${HOST_PORT}/" >/dev/null
curl -fsS "http://127.0.0.1:${HOST_PORT}/api/status"
