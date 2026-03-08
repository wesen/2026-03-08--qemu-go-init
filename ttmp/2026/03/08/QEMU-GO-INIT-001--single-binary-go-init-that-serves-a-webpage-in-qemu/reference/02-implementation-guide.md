---
Title: Implementation guide
Ticket: QEMU-GO-INIT-001
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - web
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: Makefile
      Note: Primary_build_commands
    - Path: README.md
      Note: Quick_start_and_kernel_override_notes
    - Path: cmd/mkinitramfs/main.go
      Note: Initramfs_build_steps
    - Path: scripts/qemu-smoke.sh
      Note: Smoke_test_procedure
    - Path: ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/sources/local/qemu-go-guide.md
      Note: Imported_reference_guide
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T14:22:56.675543603-04:00
WhatFor: ""
WhenToUse: ""
---


# Implementation guide

## Goal

This guide gives a new engineer a copy/paste-ready path for understanding, building, running, validating, and extending the QEMU-backed Go `/init` proof of concept.

## Context

This repository is intentionally small, but it crosses several domains at once:

- Go application development
- Linux early userspace
- initramfs packaging
- QEMU boot configuration
- HTTP service debugging

If those concepts are new, read the design document first and then use this guide as the execution checklist.

### Terms

- `PID 1`
  The first userspace process in a Linux system. If it exits unexpectedly, the system is usually no longer viable.
- `initramfs`
  A compressed cpio archive unpacked by the kernel into the initial root filesystem.
- `QEMU user networking`
  A convenient QEMU networking mode that does not require host bridge setup and supports host-to-guest port forwarding.
- `hostfwd`
  The QEMU option that maps a host port to a guest port.
- `embedded assets`
  Static files compiled into the Go binary with `go:embed`.

## Quick Reference

### Key files

| File | Why it matters |
| --- | --- |
| `cmd/init/main.go` | Defines the guest startup sequence |
| `internal/boot/boot.go` | Holds mount logic, PID 1 signal behavior, and HTTP server startup |
| `internal/webui/site.go` | Exposes `/`, `/healthz`, and `/api/status` |
| `internal/webui/static/index.html` | Embedded webpage shown in the browser |
| `cmd/mkinitramfs/main.go` | Builds the gzip-compressed initramfs |
| `internal/initramfs/writer.go` | Implements `newc` archive generation |
| `Makefile` | Main developer workflow |
| `scripts/qemu-smoke.sh` | Automated QEMU boot verification |

### Fast path commands

```bash
make test
make initramfs
KERNEL_IMAGE=/path/to/readable/bzImage make run
```

### Expected host-visible endpoints

| URL | Purpose |
| --- | --- |
| `http://127.0.0.1:8080/` | Browser UI |
| `http://127.0.0.1:8080/healthz` | Liveness probe |
| `http://127.0.0.1:8080/api/status` | Runtime JSON |

### Environment knobs

| Variable | Meaning | Default |
| --- | --- | --- |
| `KERNEL_IMAGE` | Readable kernel image to boot in QEMU | auto-detect readable `/boot/vmlinuz-*` |
| `QEMU_HOST_PORT` | Host port forwarded into guest | `8080` |
| `QEMU_GUEST_PORT` | Guest port served by `/init` | `8080` |
| `GO_INIT_HTTP_ADDR` | Listen address inside guest | `:8080` |

## Usage Examples

### 1. Build everything without booting QEMU

Use this when you want to confirm the Go code and initramfs packaging work before dealing with kernel availability.

```bash
make test
make initramfs
file build/init build/initramfs.cpio.gz
```

What success means:

- unit tests passed,
- the init binary built successfully,
- the initramfs archive exists,
- `file` reports a statically linked ELF for `build/init`.

### 2. Boot interactively with a readable kernel image

Use this when you have a readable kernel path, such as a copied `bzImage` or a distribution-provided kernel artifact outside root-only `/boot`.

```bash
KERNEL_IMAGE=/path/to/readable/bzImage make run
```

What to look for in the QEMU console:

- boot logs on the serial console,
- messages showing filesystem mount attempts,
- a log line similar to `go init ready on :8080`.

Then verify from the host:

```bash
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/api/status
```

### 3. Run the automated smoke flow

Use this when you want an unattended validation instead of manually watching the VM.

```bash
make smoke QEMU_HOST_PORT=18080 KERNEL_IMAGE=/path/to/readable/bzImage
```

What the script does:

1. Launches QEMU in the background.
2. Waits for `http://127.0.0.1:18080/healthz`.
3. Requests `/`.
4. Requests `/api/status`.
5. Exits and cleans up QEMU.

### 4. Change the guest listen address

This is mostly useful for experiments inside the VM.

```bash
GO_INIT_HTTP_ADDR=:9090 make build
```

If you change the guest port, remember to keep the QEMU `hostfwd` guest side aligned with it. The current `Makefile` assumes guest port `8080`.

## Step-by-Step Build Explanation

### Step 1: `make build`

Relevant file: `Makefile:38-40`

What happens:

- `go build` compiles `./cmd/init`.
- `CGO_ENABLED=0` forces a pure-Go static build.
- `GOOS=linux GOARCH=amd64` produces a Linux guest executable even if the host tooling later changes.
- The output lands at `build/init`.

Why it matters:

- The kernel needs a Linux userspace executable for `/init`.
- A static binary avoids surprises from missing shared libraries in the initramfs.

### Step 2: `make initramfs`

Relevant file: `cmd/mkinitramfs/main.go:64-99`

What happens:

- The tool reads `build/init`.
- It creates a gzip writer.
- It writes a `newc` archive containing directories, device nodes, and `/init`.
- The output becomes `build/initramfs.cpio.gz`.

Why it matters:

- Linux initramfs boot expects a cpio archive, not an arbitrary tarball or zip file.
- `/dev/console` and `/dev/null` are part of the minimal usable environment.

### Step 3: `make run`

Relevant file: `Makefile:22-30`

What happens:

- QEMU launches with:
  - `-nographic` so the serial console appears in the host terminal
  - `-kernel $(KERNEL_IMAGE)`
  - `-initrd build/initramfs.cpio.gz`
  - `-append "console=ttyS0 rdinit=/init ip=dhcp"`
  - `-nic user,model=virtio-net-pci,hostfwd=tcp::8080-:8080`

Why those flags matter:

- `console=ttyS0` routes output to the visible serial console.
- `rdinit=/init` tells the kernel to execute `/init` from the initramfs.
- `ip=dhcp` asks the kernel to configure networking automatically.
- `hostfwd` makes the guest HTTP service reachable from the host.

### Step 4: guest runtime initialization

Relevant files:

- `cmd/init/main.go:11-31`
- `internal/boot/boot.go:36-137`
- `internal/webui/site.go:35-73`

What happens:

1. The Go program starts as PID 1.
2. It registers signal handlers for `SIGCHLD`, `SIGINT`, and `SIGTERM`.
3. It attempts to mount `proc`, `sysfs`, and `devtmpfs`.
4. It builds the embedded site and JSON endpoint.
5. It listens forever on `:8080`.

### Step 5: browser and API inspection

Relevant file: `internal/webui/static/index.html`

What happens:

- The browser hits `/`.
- The page fetches `/api/status`.
- The page displays:
  - PID,
  - hostname,
  - Go runtime version,
  - listen address,
  - startup time,
  - uptime,
  - mount success or failure.

## Troubleshooting Guide

### Problem: `make smoke` says no readable kernel image was found

Meaning:

- The repository code is ready to boot, but the host does not provide a readable kernel image at `/boot/vmlinuz-*`.

What to do:

1. Obtain a readable kernel image path.
2. Pass it with `KERNEL_IMAGE=/path/to/bzImage`.
3. Rerun `make run` or `make smoke`.

Why this is not a code bug:

- The failure happens before the guest boots.
- The initramfs build already completed successfully.

### Problem: QEMU starts but the host cannot reach `/healthz`

Likely causes:

- the kernel did not configure networking,
- the guest never reached the HTTP server,
- or the forwarded host port is wrong.

Checks:

1. Watch the QEMU console for `go init ready on :8080`.
2. Confirm the host forwarding port matches `QEMU_HOST_PORT`.
3. Request `/api/status` to see whether the server is live but the browser UI is failing.

### Problem: the page loads but mount rows show errors

Meaning:

- `/init` is running, but one or more early filesystems did not mount.

Checks:

1. Inspect the QEMU console logs.
2. Confirm the kernel supports `proc`, `sysfs`, and `devtmpfs`.
3. Compare the mount results shown in the page with `internal/boot/mountSpecs`.

### Problem: a future change introduces child processes and zombies

Meaning:

- The current example is safe because it does not intentionally fork extra workloads, but PID 1 must always reap children.

What to inspect:

- `boot.StartChildReaper`
- `boot.reapChildren`

What to improve later:

- add more explicit process lifecycle tests,
- define shutdown semantics instead of only halting on fatal errors.

## Extension Ideas

These are sensible follow-up tasks for an intern after understanding the baseline system:

1. Add integration tests for the `/api/status` payload.
2. Add structured boot phase logging instead of plain log lines.
3. Support kernel embedding through `CONFIG_INITRAMFS_SOURCE`.
4. Add a small custom-kernel build playbook for environments with root-only `/boot`.
5. Add a richer guest status page that shows kernel command-line data or interface details.

## Related

- Design document: `design-doc/01-single-binary-go-init-architecture-and-implementation-guide.md`
- Diary: `reference/01-diary.md`
- Imported guide: `sources/local/qemu-go-guide.md`

## Related

<!-- Link to related documents or resources -->
