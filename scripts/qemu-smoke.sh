#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BUILD_DIR=${BUILD_DIR:-"${ROOT_DIR}/build"}
QEMU_BIN=${QEMU_BIN:-qemu-system-x86_64}
KERNEL_IMAGE=${KERNEL_IMAGE:-$(find /boot -maxdepth 1 -name 'vmlinuz-*' -readable 2>/dev/null | sort | tail -n 1)}
HOST_PORT=${HOST_PORT:-18080}
GUEST_PORT=${GUEST_PORT:-8080}
SSH_HOST_PORT=${SSH_HOST_PORT:-10022}
SSH_GUEST_PORT=${SSH_GUEST_PORT:-2222}
QEMU_MEMORY=${QEMU_MEMORY:-512}
QEMU_APPEND=${QEMU_APPEND:-console=ttyS0 rdinit=/init}
QEMU_LOG=${QEMU_LOG:-"${BUILD_DIR}/qemu-smoke.log"}
QEMU_NET_MODEL=${QEMU_NET_MODEL:-virtio-net-pci}
QEMU_PCAP=${QEMU_PCAP:-}
QEMU_ENABLE_VIRTIO_RNG=${QEMU_ENABLE_VIRTIO_RNG:-1}
QEMU_RNG_OBJECT=${QEMU_RNG_OBJECT:-rng-random,id=rng0,filename=/dev/urandom}
QEMU_RNG_DEVICE=${QEMU_RNG_DEVICE:-virtio-rng-pci,rng=rng0}
QEMU_REQUIRE_VIRTIO_RNG=${QEMU_REQUIRE_VIRTIO_RNG:-1}

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
if [[ -n "${QEMU_PCAP}" ]]; then
  rm -f "${QEMU_PCAP}"
fi

QEMU_ARGS=(
  -m "${QEMU_MEMORY}"
  -nographic
  -no-reboot
  -kernel "${KERNEL_IMAGE}"
  -initrd "${BUILD_DIR}/initramfs.cpio.gz"
  -append "${QEMU_APPEND}"
)

if [[ -n "${QEMU_PCAP}" ]]; then
  QEMU_ARGS+=(
    -netdev "user,id=n1,hostfwd=tcp::${HOST_PORT}-:${GUEST_PORT},hostfwd=tcp::${SSH_HOST_PORT}-:${SSH_GUEST_PORT}"
    -device "${QEMU_NET_MODEL},netdev=n1"
    -object "filter-dump,id=f1,netdev=n1,file=${QEMU_PCAP}"
  )
else
  QEMU_ARGS+=(
    -nic "user,model=${QEMU_NET_MODEL},hostfwd=tcp::${HOST_PORT}-:${GUEST_PORT},hostfwd=tcp::${SSH_HOST_PORT}-:${SSH_GUEST_PORT}"
  )
fi

case "${QEMU_ENABLE_VIRTIO_RNG,,}" in
  1|true|yes|on)
    QEMU_ARGS+=(
      -object "${QEMU_RNG_OBJECT}"
      -device "${QEMU_RNG_DEVICE}"
    )
    QEMU_VIRTIO_RNG=enabled
    ;;
  *)
    QEMU_VIRTIO_RNG=disabled
    ;;
esac

echo "qemu-smoke: kernel=${KERNEL_IMAGE} http_host_port=${HOST_PORT} http_guest_port=${GUEST_PORT} ssh_host_port=${SSH_HOST_PORT} ssh_guest_port=${SSH_GUEST_PORT} model=${QEMU_NET_MODEL} pcap=${QEMU_PCAP:-disabled} virtio_rng=${QEMU_VIRTIO_RNG}" >"${QEMU_LOG}"
"${QEMU_BIN}" "${QEMU_ARGS[@]}" >>"${QEMU_LOG}" 2>&1 &
QEMU_PID=$!

cleanup() {
  if kill -0 "${QEMU_PID}" 2>/dev/null; then
    kill "${QEMU_PID}" 2>/dev/null || true
    wait "${QEMU_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

for _ in $(seq 1 80); do
  if curl -fsS --max-time 1 "http://127.0.0.1:${HOST_PORT}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done

curl -fsS --max-time 5 "http://127.0.0.1:${HOST_PORT}/" >/dev/null
STATUS_JSON=$(curl -fsS --max-time 5 "http://127.0.0.1:${HOST_PORT}/api/status")
printf '%s\n' "${STATUS_JSON}"

set +e
SSH_OUTPUT=$(timeout 10s ssh -tt \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o PreferredAuthentications=none \
  -o PubkeyAuthentication=no \
  -o PasswordAuthentication=no \
  -o ConnectTimeout=5 \
  -p "${SSH_HOST_PORT}" \
  127.0.0.1 2>&1 </dev/null)
SSH_EXIT=$?
set -e

printf '\n%s\n' "${SSH_OUTPUT}"

if [[ "${SSH_EXIT}" -ne 0 ]]; then
  echo "ssh smoke failed with exit ${SSH_EXIT}" >&2
  exit "${SSH_EXIT}"
fi

printf '%s\n' "${SSH_OUTPUT}" | rg -q 'qemu-go-init / wish'
printf '%s\n' "${SSH_OUTPUT}" | rg -q 'Host key'

case "${QEMU_REQUIRE_VIRTIO_RNG,,}" in
  1|true|yes|on)
    printf '%s\n' "${STATUS_JSON}" | rg -q '"virtioRngVisible": true'
    ;;
esac

printf '%s\n' "${STATUS_JSON}" | rg -q '"started": true'
