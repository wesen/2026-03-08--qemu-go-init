#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 /path/to/elf" >&2
  exit 1
fi

BIN=$1
if [[ ! -f "${BIN}" ]]; then
  echo "binary not found: ${BIN}" >&2
  exit 1
fi

declare -A seen=()

emit_mapping() {
  local guest_path=$1
  local host_path=$2

  [[ -n "${guest_path}" ]] || return 0
  [[ -n "${host_path}" ]] || return 0

  host_path=$(readlink -f "${host_path}")
  if [[ ! -f "${host_path}" ]]; then
    echo "runtime dependency not found: ${host_path}" >&2
    exit 1
  fi

  local key="${guest_path}=${host_path}"
  if [[ -n "${seen[${key}]:-}" ]]; then
    return 0
  fi
  seen["${key}"]=1
  printf '%s\n' "${key}"
}

while IFS= read -r line; do
  line=${line#"${line%%[![:space:]]*}"}
  [[ -n "${line}" ]] || continue
  [[ "${line}" == linux-vdso.so.1* ]] && continue

  if [[ "${line}" == *"=>"* ]]; then
    guest_path=${line%% =>*}
    rest=${line#*=> }
    host_path=${rest%% (*}
    [[ "${host_path}" == "not found" ]] && {
      echo "unresolved runtime dependency for ${guest_path}" >&2
      exit 1
    }
    if [[ "${guest_path}" != /* ]]; then
      guest_path=${host_path}
    fi
    emit_mapping "${guest_path}" "${host_path}"
    continue
  fi

  if [[ "${line}" == /*" ("* ]]; then
    guest_path=${line%% (*}
    emit_mapping "${guest_path}" "${guest_path}"
  fi
done < <(ldd "${BIN}")
