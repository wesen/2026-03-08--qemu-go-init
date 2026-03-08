---
Title: Implementation guide
Ticket: QEMU-GO-INIT-003
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - web
    - networking
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/mkinitramfs/main.go
      Note: step-by-step builder flow for module decompression and archive packaging
    - Path: internal/kmod/kmod.go
      Note: step-by-step finit_module wrapper for virtio_rng activation
    - Path: scripts/qemu-smoke.sh
      Note: operator validation path that asserts virtioRngVisible
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T16:35:36.322849851-04:00
WhatFor: ""
WhenToUse: ""
---


# Implementation guide

## Goal

Provide a concrete, file-level implementation guide for the final entropy-support design that now exists in the repo. This guide is written for a new intern who needs to understand not just what changed, but how the build path, boot path, and runtime status path fit together.

## System Summary

The repo now solves early-boot entropy in four layers:

1. QEMU exposes a `virtio-rng` device.
2. The initramfs builder embeds the matching `virtio_rng` kernel module for the booted distro kernel.
3. The Go PID 1 runtime loads that module during boot and waits for the kernel to report a visible RNG backend.
4. The status API and webpage expose the resulting entropy state and module-loading result.

## Architecture Map

### Build-time files

- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
  - Decides whether `virtio-rng` is exposed in QEMU.
  - Decides whether the initramfs should embed the host `virtio_rng` module.
- [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go)
  - Reads `build/init`.
  - Reads the host kernel module from `/lib/modules/.../virtio-rng.ko.zst`.
  - Decompresses `.zst` content into an ELF `.ko`.
  - Writes `/init` and `/lib/modules/virtio_rng.ko` into the `newc` archive.
- [internal/initramfs/writer.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/initramfs/writer.go)
  - Serializes the actual `newc` archive entries.

### Boot-time files

- [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
  - Defines early-boot ordering.
- [internal/boot/boot.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/boot/boot.go)
  - Mounts `/proc`, `/sys`, and `/dev`.
- [internal/kmod/kmod.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/kmod/kmod.go)
  - Loads `/lib/modules/virtio_rng.ko` with `finit_module`.
- [internal/entropy/entropy.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy.go)
  - Reads `entropy_avail`, `rng_current`, `rng_available`, and `/dev/hwrng`.
  - Waits briefly after module load until `virtio_rng` becomes visible.
- [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go)
  - Configures guest IPv4 networking.

### Operator-facing files

- [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
  - Adds `entropy` and `virtioRngModule` to `/api/status`.
- [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html)
  - Renders the entropy panel with kernel-module and RNG-backend status.
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
  - Boots the guest and now fails if `virtioRngVisible` is not `true`.

## Control Flow Diagram

```text
make initramfs
  |
  v
cmd/mkinitramfs
  |- read build/init
  |- read host virtio-rng.ko.zst
  |- decompress to ELF .ko
  `- write initramfs.cpio.gz

make run / make smoke
  |
  v
QEMU
  |- expose virtio-net
  `- expose virtio-rng

guest /init
  |- mount proc/sys/dev
  |- load /lib/modules/virtio_rng.ko
  |- wait until rng_current/rng_available report virtio
  |- configure networking
  |- probe final entropy state
  `- serve web UI + /api/status
```

## Implementation Details

### QEMU side

The QEMU change is intentionally simple:

```bash
-object rng-random,id=rng0,filename=/dev/urandom
-device virtio-rng-pci,rng=rng0
```

Those flags are controlled in [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile) and [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh).

### Initramfs side

The build-time insight that matters most is this: the host only provides `virtio-rng.ko.zst`, but the first runtime attempt proved that the guest `finit_module` path did not accept the compressed bytes directly. The initramfs builder therefore has to unpack the module before boot.

Pseudocode:

```go
moduleData := readModuleData("/lib/modules/.../virtio-rng.ko.zst")
// readModuleData decompresses .zst to ELF .ko bytes

archive.AddDirectory("lib", 0o755, modTime)
archive.AddDirectory("lib/modules", 0o755, modTime)
archive.AddFile("lib/modules/virtio_rng.ko", 0o644, modTime, moduleData)
```

### Runtime side

The boot order in [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go) matters:

```go
results := boot.PrepareFilesystem(logger)
moduleResult := kmod.LoadVirtioRNG(logger)
entropyResult := entropy.Probe(logger)
if moduleResult.Loaded {
    entropyResult = entropy.WaitForVirtioRNG(logger, 2*time.Second)
}
networkResult, err := networking.Configure(logger)
handler, err := webui.NewHandler(webui.Options{
    Mounts:          results,
    Network:         networkResult,
    Entropy:         entropyResult,
    VirtioRNGModule: moduleResult,
})
```

Why this order:

- `/sys` and `/dev` must exist before probing or loading.
- The module should load before the rest of userspace starts doing work that may rely on randomness.
- The wait loop makes the guest status deterministic instead of depending on a race between module registration and the first probe.

## API Contract

The final JSON status payload now includes:

```json
{
  "entropy": {
    "entropyAvail": 256,
    "entropyAvailKnown": true,
    "hwrngDevice": true,
    "rngCurrent": "virtio_rng.0",
    "rngAvailable": ["virtio_rng.0"],
    "virtioRngVisible": true
  },
  "virtioRngModule": {
    "attempted": true,
    "loaded": true,
    "modulePath": "/lib/modules/virtio_rng.ko",
    "step": "loaded"
  }
}
```

Interpretation:

- `entropy.virtioRngVisible` is the real outcome signal.
- `virtioRngModule` explains how the guest got there or why it did not.

## Failure Modes and How to Debug Them

### 1. QEMU is not exposing the device

Symptoms:

- `virtioRngVisible` stays `false`
- module loading may still succeed, but no backend appears

Check:

```bash
qemu-system-x86_64 -device help | rg 'virtio-rng-pci'
```

### 2. Module file missing from initramfs

Symptoms:

- `virtioRngModule.step` becomes `missing`
- `virtioRngModule.error` says the file does not exist

Check:

- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
- [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go)

### 3. Compressed module passed directly to `finit_module`

Symptoms:

- guest log shows `Invalid ELF header magic`
- status shows `exec format error`

This happened during implementation and is why the builder now decompresses `.ko.zst` before packaging.

### 4. Kernel/module version mismatch

Symptoms:

- `finit_module` fails with invalid format or vermagic-related errors

Check:

```bash
modinfo -F vermagic virtio_rng
strings /tmp/qemu-vmlinuz | rg '6\\.8\\.0-101-generic'
```

## Validation Workflow

Run these in order:

```bash
make test
make initramfs
timeout 90s env KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 make smoke
curl -fsS http://127.0.0.1:18080/api/status
```

Expected final state:

- the smoke script exits `0`
- `/api/status` shows:
  - `virtioRngVisible: true`
  - `rngCurrent: "virtio_rng.0"`
  - `virtioRngModule.loaded: true`

## Review Checklist

- Confirm [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go) converts `.ko.zst` input into guest `/lib/modules/virtio_rng.ko`.
- Confirm [internal/kmod/kmod.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/kmod/kmod.go) treats `EEXIST` as already-loaded success.
- Confirm [internal/entropy/entropy.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy.go) waits for visible activation after a successful module load.
- Confirm [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh) asserts `virtioRngVisible`.

## Related

- [01-early-boot-entropy-support-architecture-and-implementation-guide-for-the-qemu-go-init-runtime.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/design-doc/01-early-boot-entropy-support-architecture-and-implementation-guide-for-the-qemu-go-init-runtime.md)
- [01-diary.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/reference/01-diary.md)
