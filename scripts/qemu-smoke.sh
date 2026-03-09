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
QEMU_IMPORT_HOST_LOGS=${QEMU_IMPORT_HOST_LOGS:-1}
QEMU_NET_MODEL=${QEMU_NET_MODEL:-virtio-net-pci}
QEMU_PCAP=${QEMU_PCAP:-}
QEMU_ENABLE_STORAGE=${QEMU_ENABLE_STORAGE:-1}
QEMU_DATA_IMAGE=${QEMU_DATA_IMAGE:-"${BUILD_DIR}/data.img"}
QEMU_ENABLE_SHARED_STATE=${QEMU_ENABLE_SHARED_STATE:-1}
QEMU_SHARED_STATE_HOST_PATH=${QEMU_SHARED_STATE_HOST_PATH:-"${BUILD_DIR}/shared-state"}
QEMU_SHARED_STATE_MOUNT_TAG=${QEMU_SHARED_STATE_MOUNT_TAG:-hostshare}
QEMU_SHARED_STATE_FSDEV_ID=${QEMU_SHARED_STATE_FSDEV_ID:-hostsharefs}
QEMU_HOST_LOG_DB=${QEMU_HOST_LOG_DB:-"${QEMU_SHARED_STATE_HOST_PATH}/chat/qemu-host-logs.db"}
QEMU_ENABLE_VIRTIO_RNG=${QEMU_ENABLE_VIRTIO_RNG:-1}
QEMU_RNG_OBJECT=${QEMU_RNG_OBJECT:-rng-random,id=rng0,filename=/dev/urandom}
QEMU_RNG_DEVICE=${QEMU_RNG_DEVICE:-virtio-rng-pci,rng=rng0}
QEMU_REQUIRE_VIRTIO_RNG=${QEMU_REQUIRE_VIRTIO_RNG:-1}
QEMU_REQUIRE_STORAGE=${QEMU_REQUIRE_STORAGE:-1}
QEMU_REQUIRE_SHARED_STATE=${QEMU_REQUIRE_SHARED_STATE:-1}
QEMU_PID=

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

case "${QEMU_ENABLE_STORAGE,,}" in
  1|true|yes|on)
    if [[ ! -f "${QEMU_DATA_IMAGE}" ]]; then
      echo "missing QEMU_DATA_IMAGE=${QEMU_DATA_IMAGE}; run 'make data-image' first" >&2
      exit 1
    fi
    ;;
esac

mkdir -p "${BUILD_DIR}"
mkdir -p "${QEMU_SHARED_STATE_HOST_PATH}"
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

case "${QEMU_ENABLE_STORAGE,,}" in
  1|true|yes|on)
    QEMU_ARGS+=(
      -drive "file=${QEMU_DATA_IMAGE},if=virtio,format=raw"
    )
    QEMU_STORAGE=enabled
    ;;
  *)
    QEMU_STORAGE=disabled
    ;;
esac

case "${QEMU_ENABLE_SHARED_STATE,,}" in
  1|true|yes|on)
    QEMU_ARGS+=(
      -virtfs "local,path=${QEMU_SHARED_STATE_HOST_PATH},mount_tag=${QEMU_SHARED_STATE_MOUNT_TAG},security_model=none,id=${QEMU_SHARED_STATE_FSDEV_ID}"
      -device "virtio-9p-pci,fsdev=${QEMU_SHARED_STATE_FSDEV_ID},mount_tag=${QEMU_SHARED_STATE_MOUNT_TAG}"
    )
    QEMU_SHARED=enabled
    ;;
  *)
    QEMU_SHARED=disabled
    ;;
esac

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

echo "qemu-smoke: kernel=${KERNEL_IMAGE} http_host_port=${HOST_PORT} http_guest_port=${GUEST_PORT} ssh_host_port=${SSH_HOST_PORT} ssh_guest_port=${SSH_GUEST_PORT} storage=${QEMU_STORAGE} data_image=${QEMU_DATA_IMAGE} shared=${QEMU_SHARED} shared_path=${QEMU_SHARED_STATE_HOST_PATH} model=${QEMU_NET_MODEL} pcap=${QEMU_PCAP:-disabled} virtio_rng=${QEMU_VIRTIO_RNG}" >"${QEMU_LOG}"

cleanup() {
  if [[ -n "${QEMU_PID}" ]] && kill -0 "${QEMU_PID}" 2>/dev/null; then
    kill "${QEMU_PID}" 2>/dev/null || true
    wait "${QEMU_PID}" 2>/dev/null || true
    QEMU_PID=
  fi
}

import_qemu_log() {
  case "${QEMU_IMPORT_HOST_LOGS,,}" in
    1|true|yes|on)
      if [[ -f "${QEMU_LOG}" ]]; then
        mkdir -p "$(dirname "${QEMU_HOST_LOG_DB}")"
        (cd "${ROOT_DIR}" && go run ./cmd/importqemulogs -input "${QEMU_LOG}" -db "${QEMU_HOST_LOG_DB}") >/dev/null
      fi
      ;;
  esac
}

finish() {
  cleanup
  import_qemu_log
}
trap finish EXIT

start_vm() {
  local label=$1
  cleanup
  printf '\n=== %s ===\n' "${label}" >>"${QEMU_LOG}"
  "${QEMU_BIN}" "${QEMU_ARGS[@]}" >>"${QEMU_LOG}" 2>&1 &
  QEMU_PID=$!
}

wait_for_http() {
  for _ in $(seq 1 80); do
    if curl -fsS --max-time 1 "http://127.0.0.1:${HOST_PORT}/healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

fetch_status() {
  curl -fsS --max-time 5 "http://127.0.0.1:${HOST_PORT}/" >/dev/null
  curl -fsS --max-time 5 "http://127.0.0.1:${HOST_PORT}/api/status"
}

run_ssh_session() {
  set +e
  local output
  output=$({ sleep 1; printf 'x6*7\r'; sleep 1; printf '\002q'; } | timeout 12s ssh -tt \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -o PreferredAuthentications=none \
    -o PubkeyAuthentication=no \
    -o PasswordAuthentication=no \
    -o ConnectTimeout=5 \
    -o LogLevel=ERROR \
    -p "${SSH_HOST_PORT}" \
    127.0.0.1 2>&1)
  local exit_code=$?
  set -e

  printf '%s\n' "${output}"
  if [[ "${exit_code}" -ne 0 ]]; then
    echo "ssh smoke failed with exit ${exit_code}" >&2
    exit "${exit_code}"
  fi
}

scan_host_key() {
  local scan
  for _ in $(seq 1 20); do
    scan=$(ssh-keyscan -T 2 -p "${SSH_HOST_PORT}" 127.0.0.1 2>/dev/null | awk 'NF >= 3 { print $2" "$3; exit }')
    if [[ -n "${scan}" ]]; then
      printf '%s\n' "${scan}"
      return 0
    fi
    sleep 0.5
  done
  return 1
}

assert_status() {
  local json=$1

  printf '%s\n' "${json}" | rg -U -P -q '"ssh":\s*\{[\s\S]*?"started":\s*true'
  case "${QEMU_REQUIRE_STORAGE,,}" in
    1|true|yes|on)
      printf '%s\n' "${json}" | rg -U -P -q '"storage":\s*\{[\s\S]*?"mounted":\s*true'
      ;;
  esac
  case "${QEMU_REQUIRE_VIRTIO_RNG,,}" in
    1|true|yes|on)
      printf '%s\n' "${json}" | rg -q '"virtioRngVisible": true'
      ;;
  esac
  case "${QEMU_REQUIRE_SHARED_STATE,,}" in
    1|true|yes|on)
      printf '%s\n' "${json}" | rg -U -P -q '"sharedState":\s*\{[\s\S]*?"mounted":\s*true[\s\S]*?"step":\s*"ready"'
      ;;
  esac
}

probe_boot() {
  local label=$1
  start_vm "${label}"
  wait_for_http

  local status_json
  status_json=$(fetch_status)
  printf '%s\n' "${status_json}" >&2
  assert_status "${status_json}"

  local ssh_output
  ssh_output=$(run_ssh_session)
  printf '\n%s\n' "${ssh_output}" >&2
  printf '%s\n' "${ssh_output}" | rg -q 'qemu-go-init bbs'
  printf '%s\n' "${ssh_output}" | rg -q 'Shared-state Bubble Tea board|SSH BBS'
  printf '%s\n' "${ssh_output}" | rg -q '/var/lib/go-init/shared/bbs'
  printf '%s\n' "${ssh_output}" | rg -q 'qemu-go-init JavaScript REPL'
  printf '%s\n' "${ssh_output}" | rg -q '\b42\b'

  local host_key
  host_key=$(scan_host_key)
  if [[ -z "${host_key}" ]]; then
    echo "failed to scan SSH host key" >&2
    exit 1
  fi

  cleanup
  sleep 1
  printf '%s\n' "${host_key}"
}

FIRST_OUTPUT=$(probe_boot "boot-1")
FIRST_KEY=$(printf '%s\n' "${FIRST_OUTPUT}" | tail -n 1)
SECOND_OUTPUT=$(probe_boot "boot-2")
SECOND_KEY=$(printf '%s\n' "${SECOND_OUTPUT}" | tail -n 1)

printf '\nfirst host key:  %s\n' "${FIRST_KEY}"
printf 'second host key: %s\n' "${SECOND_KEY}"

if [[ "${FIRST_KEY}" != "${SECOND_KEY}" ]]; then
  echo "persistent host key mismatch across reboot" >&2
  exit 1
fi
