---
Title: Full system architecture, usage, and extension guide for qemu-go-init
Ticket: QEMU-GO-INIT-003
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - web
    - networking
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: Makefile
      Note: primary build
    - Path: README.md
      Note: operator entrypoint and supported environment variables
    - Path: cmd/init/main.go
      Note: guest PID 1 orchestration order
    - Path: cmd/mkinitramfs/main.go
      Note: host-side initramfs assembly and module decompression
    - Path: internal/boot/boot.go
      Note: mounting and PID 1 lifecycle helpers
    - Path: internal/entropy/entropy.go
      Note: entropy probe and post-load wait loop
    - Path: internal/initramfs/writer.go
      Note: minimal newc archive writer used by the builder
    - Path: internal/kmod/kmod.go
      Note: kernel module activation path for virtio_rng
    - Path: internal/networking/network.go
      Note: userspace DHCP and network configuration flow
    - Path: internal/webui/site.go
      Note: HTTP status API contract
    - Path: internal/webui/static/index.html
      Note: browser-side status rendering
    - Path: scripts/qemu-smoke.sh
      Note: end-to-end QEMU validation and API assertion flow
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T17:09:29.422693698-04:00
WhatFor: ""
WhenToUse: ""
---


# Full system architecture, usage, and extension guide for qemu-go-init

## Executive Summary

`qemu-go-init` is a deliberately small Linux boot demo whose core claim is simple: a single statically linked Go binary can act as PID 1 inside a QEMU guest, bring up enough of the system to be useful, and serve a browser-visible webpage plus a JSON status API. The system is intentionally not a general-purpose distro. It is a tightly scoped, rootless build and boot pipeline designed to demonstrate early-boot control, userspace networking, and guest observability with as little moving machinery as possible.

Today, the system has five major layers:

1. A host-side build layer that compiles a static `/init` binary and packs it into a `newc` initramfs archive.
2. A QEMU launch layer that boots a chosen Linux kernel with that initramfs and forwards a host TCP port into the guest.
3. A PID 1 runtime layer that mounts the minimum filesystems, loads the `virtio_rng` module, configures guest networking, and starts an HTTP server.
4. A status/reporting layer that exposes mount, network, and entropy state as structured JSON.
5. An operator/developer workflow layer that provides `make` targets, a smoke script, and documentation for validation and extension.

For a new intern, the most important conceptual model is this:

- The Linux kernel does the earliest boot work and then executes `/init`.
- In this repo, `/init` is the Go binary built from [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go#L15).
- That binary is responsible for becoming a minimal operating environment, not just for serving web requests.
- If boot ordering is wrong, the system fails long before the webpage exists.

This document explains the current system as it exists in the repository now, how to run it, how to inspect it, and how to extend it safely.

## Problem Statement

The repository needs documentation that covers the whole system, not just isolated feature tickets. The existing docs explain individual slices such as userspace DHCP and entropy support, but a new engineer still has to reconstruct the larger architecture from code and commit history.

That creates avoidable onboarding friction:

- A reader may understand that `make run` boots something, but not how [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile#L27) relates to [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go#L42), [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go#L15), and [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh#L41).
- A reader may see that networking works, but not realize that it is implemented in userspace with raw DHCP and netlink rather than kernel `ip=dhcp` support.
- A reader may see entropy diagnostics in the UI, but not understand the build-time and boot-time steps that activate `virtio_rng`.
- A reader may know how to change one file, but not know which invariants must remain true across build, boot, API, and smoke validation.

The problem this document addresses is therefore not a code defect. It is an architecture comprehension defect. The goal is to remove that gap.

## Proposed Solution

Provide one comprehensive, evidence-backed system guide for the current repository. The guide should:

1. Map the system from host build to guest HTTP response.
2. Name the key files and packages, with concrete file references.
3. Explain the boot sequence in the order it actually occurs.
4. Document the important API contracts and environment variables.
5. Show how the smoke path validates the system.
6. Explain how to extend the system without accidentally breaking boot.

The rest of this document is the solution.

## Scope And Audience

This guide is written for:

- a new intern joining the project,
- an engineer reviewing the repo for the first time,
- a maintainer who needs one place to remember the system invariants.

This guide is not intended to teach Linux from first principles. It assumes the reader can follow Go code and basic shell commands, but it does not assume prior experience with:

- PID 1 behavior,
- initramfs structure,
- QEMU user networking,
- Linux module loading,
- early-boot entropy pitfalls.

## System At A Glance

The current architecture can be summarized like this:

```text
Host workstation
  |
  | make build / make initramfs / make run / make smoke
  v
Go compiler + initramfs builder
  |
  | build/init
  | build/initramfs.cpio.gz
  v
QEMU + Linux kernel
  |
  | kernel boots
  | kernel runs /init from initramfs
  v
Go PID 1 runtime
  |
  | mount /proc /sys /dev
  | load virtio_rng module
  | probe entropy
  | configure networking
  | serve HTTP
  v
Embedded webpage + /api/status
  |
  | host port forwarding
  v
Browser on host
```

Two design choices make the repo easy to reason about:

- The runtime is mostly self-contained in Go rather than shelling out to BusyBox tools.
- The host-side automation is intentionally thin and readable.

## Repository Map

The most important files are:

- [README.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/README.md#L1)
  - Human entrypoint for running the system.
- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile#L1)
  - Primary build and run interface.
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh#L1)
  - End-to-end validation harness.
- [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go#L15)
  - Guest entrypoint; this becomes `/init`.
- [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go#L42)
  - Host-side initramfs builder.
- [internal/boot/boot.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/boot/boot.go#L29)
  - Mounts and PID 1 helpers.
- [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go#L98)
  - Userspace DHCP, netlink configuration, fallback logic.
- [internal/kmod/kmod.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/kmod/kmod.go#L27)
  - Early-boot kernel module loading.
- [internal/entropy/entropy.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy.go#L30)
  - Entropy visibility and wait loop.
- [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go#L44)
  - HTTP mux and JSON API.
- [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html#L211)
  - Browser-side status renderer.
- [internal/initramfs/writer.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/initramfs/writer.go#L36)
  - Minimal `newc` cpio archive writer.

## How The Build Path Works

### 1. `make build` compiles the guest binary

[Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile#L49) builds the guest runtime with:

```make
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags='-s -w' -o $(INIT_BIN) ./cmd/init
```

Important implications:

- `CGO_ENABLED=0` keeps the binary statically linked and easier to run in a tiny initramfs.
- `GOOS=linux` and `GOARCH=amd64` target the QEMU guest platform, not necessarily the host runtime environment.
- The resulting file becomes `build/init`, which is later archived as `/init` inside the initramfs.

### 2. `make initramfs` packs the runtime into a bootable archive

[Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile#L53) invokes [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go#L27).

The builder does four important things:

1. Reads the already-built `build/init` binary.
2. Optionally reads the host `virtio_rng` module from `INITRAMFS_VIRTIO_RNG_MODULE_SRC`.
3. Decompresses `.zst` module input to raw ELF bytes when necessary.
4. Writes a gzip-compressed `newc` archive containing:
   - `init`
   - `dev/console`
   - `dev/null`
   - `proc`
   - `sys`
   - `lib/modules/virtio_rng.ko` when enabled

The module handling is implemented in [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go#L82).

Pseudocode:

```go
initData := readFile(initPath)
extras := readExtraFiles(virtioRNGModuleSrc)

archive.AddDirectory("dev")
archive.AddDirectory("proc")
archive.AddDirectory("sys")
archive.AddCharDevice("dev/console")
archive.AddCharDevice("dev/null")
archive.AddFile("init", initData)

for each extra file:
    add parent directories
    add file bytes
```

### 3. `internal/initramfs` is intentionally minimal

[internal/initramfs/writer.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/initramfs/writer.go#L15) does not try to be a general archive library. It only knows how to emit the `newc` cpio format this repo needs.

That narrow design keeps the repo understandable:

- `AddDirectory` writes a directory entry.
- `AddFile` writes a regular file entry.
- `AddCharDevice` writes device nodes like `/dev/console`.
- `Close` writes the `TRAILER!!!` record required by `newc`.

This is a good example of a deliberate design tradeoff: the code is small because the problem is tightly scoped.

## How The QEMU Launch Path Works

[Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile#L27) defines the `run` target. It boots:

- a chosen Linux kernel via `-kernel $(KERNEL_IMAGE)`,
- the generated initramfs via `-initrd $(INITRAMFS)`,
- and a forwarded host port via `-nic user,model=virtio-net-pci,hostfwd=tcp::HOST-:GUEST`.

It also conditionally adds:

```text
-object rng-random,id=rng0,filename=/dev/urandom
-device virtio-rng-pci,rng=rng0
```

when `QEMU_ENABLE_VIRTIO_RNG` is enabled.

This means the runtime depends on three distinct host-side inputs:

- a readable kernel image,
- the generated initramfs,
- the QEMU command-line topology.

If any of those drift out of sync, the guest may still boot but not behave the way the repo expects.

## How The Guest Boot Path Works

The guest boot path starts in [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go#L15). The order is critical:

```go
boot.StartChildReaper(logger)
results := boot.PrepareFilesystem(logger)
moduleResult := kmod.LoadVirtioRNG(logger)
entropyResult := entropy.Probe(logger)
if moduleResult.Loaded {
    entropyResult = entropy.WaitForVirtioRNG(logger, 2*time.Second)
}
networkResult, err := networking.Configure(logger)
handler, err := webui.NewHandler(...)
boot.ServeHTTP(addr, handler, logger)
```

### Why this order matters

1. The process must behave like PID 1.
   - [internal/boot/boot.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/boot/boot.go#L52) installs a child reaper and signal handling.
2. `/proc`, `/sys`, and `/dev` must exist before probing hardware or loading modules.
   - [internal/boot/boot.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/boot/boot.go#L29)
3. The entropy driver should load before later code depends on randomness.
4. Networking must be configured before the host can reach the webpage.
5. The HTTP server is the last step because it is the human-visible confirmation that the rest of boot worked.

## Package-By-Package Explanation

### `internal/boot`

[internal/boot/boot.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/boot/boot.go#L12) is the PID 1 support package.

What it does:

- Defines a `MountResult` contract that is later surfaced in the API.
- Mounts `proc`, `sysfs`, and `devtmpfs`.
- Provides `HTTPAddress()` to read `GO_INIT_HTTP_ADDR`.
- Starts a child reaper so the PID 1 process does not leak zombies.
- Wraps the HTTP handler with request logging.
- Provides `Halt()` to keep PID 1 resident if boot reaches a fatal state.

The most important conceptual point for an intern:

- `Halt()` is not a normal application pattern.
- It exists because if PID 1 exits in an initramfs-only boot, the whole guest can collapse in confusing ways.

### `internal/kmod`

[internal/kmod/kmod.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/kmod/kmod.go#L11) is responsible for activating the guest entropy driver from the packaged initramfs module.

Important behaviors:

- It expects the guest module path to be `/lib/modules/virtio_rng.ko`.
- It uses `unix.FinitModule`.
- It treats `EEXIST` as success, because “already loaded” is good enough for boot.
- It returns structured status rather than just logging a string.

That structured result is what eventually appears in `/api/status` as `virtioRngModule`.

### `internal/entropy`

[internal/entropy/entropy.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy.go#L20) is a read-only probe package.

It inspects:

- `/proc/sys/kernel/random/entropy_avail`
- `/sys/class/misc/hw_random/rng_available`
- `/sys/class/misc/hw_random/rng_current`
- `/dev/hwrng`

Its responsibilities are:

- report current entropy state,
- infer whether `virtio-rng` is actually visible,
- wait briefly after module load so the API reflects the real post-activation state.

This wait loop exists because hardware visibility is not always instantaneous from the perspective of userspace code.

### `internal/networking`

[internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go#L32) is the most systems-heavy package in the repo.

What it does:

1. Reads config from environment variables.
2. Chooses a non-loopback interface.
3. Brings the link up with netlink.
4. Opens a raw DHCP socket on that interface.
5. Runs DHCP in userspace.
6. Applies address and route information with netlink.
7. Writes `/etc/resolv.conf`.
8. Falls back to the standard QEMU user-network static defaults when configured to do so.

The key lesson here is that the repo does not depend on the kernel understanding `ip=dhcp`. It has its own userspace networking path.

Pseudocode:

```go
cfg := LoadConfigFromEnv()
link := selectLink(cfg.PreferredInterface)
netlink.LinkSetUp(link)
conn := nclient4.NewRawUDPConn(linkName, clientPort)
client := nclient4.NewWithConn(conn, mac, timeout, retry)
lease := requestLease(ctx, client, mac, xid)
details := detailsFromLease(lease)
applyLease(link, details)
writeResolvConf(details.ResolverContents)
```

The `Result` type in [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go#L32) is also part of the public internal contract of the system, because it is embedded directly in the JSON API.

### `internal/webui`

[internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go#L21) turns system state into browser-visible output.

There are three important pieces:

1. `Options`
   - the structured inputs passed from `cmd/init`.
2. `statusResponse`
   - the JSON contract served from `/api/status`.
3. Embedded static files
   - served directly from Go via `go:embed`.

The browser code in [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html#L211) polls `/api/status` every five seconds and renders:

- boot/mount status,
- network status,
- entropy status.

This is intentionally simple. The UI is a thin view over the JSON status contract.

## Boot Sequence Diagram

```text
Kernel boots
  |
  v
exec /init
  |
  v
StartChildReaper
  |
  v
PrepareFilesystem
  |- mount /proc
  |- mount /sys
  `- mount /dev
  |
  v
LoadVirtioRNG
  |
  v
Probe / WaitForVirtioRNG
  |
  v
Configure networking
  |
  v
Build HTTP handler
  |
  v
Listen on :8080
```

## API Contract

### `/healthz`

[internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go#L73) returns `200 OK` with `ok\n`.

This is intentionally minimal. It answers only “is the HTTP process responding?”

### `/api/status`

[internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go#L77) returns a pretty-printed JSON document with:

- process/runtime metadata,
- mount results,
- network result,
- entropy result,
- module-loading result.

Representative shape:

```json
{
  "pid": 1,
  "listenAddr": ":8080",
  "mounts": [...],
  "network": {
    "method": "userspace-dhcp",
    "configured": true,
    "interfaceName": "eth0",
    "cidr": "10.0.2.15/24",
    "gateway": "10.0.2.2"
  },
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

### Contract advice for future changes

If you extend `statusResponse`, do these together:

1. Add the new field in [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go#L29).
2. Populate it from `webui.Options`.
3. Update [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html#L211) if the browser should render it.
4. Update smoke validation if the new field is part of a required boot invariant.
5. Update docs so the field is not “tribal knowledge.”

## How To Use The System

### Quick-start workflow

From [README.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/README.md#L7):

```bash
make test
make run
```

Then browse:

```text
http://127.0.0.1:8080/
```

If `/boot/vmlinuz-*` is unreadable, use a readable kernel copy:

```bash
KERNEL_IMAGE=/tmp/qemu-vmlinuz make run
```

### End-to-end validation workflow

Use the smoke path when you want an assertion, not just a manual run:

```bash
make test
make initramfs
timeout 90s env KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 make smoke
```

What the smoke path guarantees:

- the guest booted,
- `/healthz` responded,
- `/api/status` responded,
- the current configuration reported `virtioRngVisible: true`.

### Debugging with packet capture

[scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh#L50) supports `QEMU_PCAP` to enable QEMU `filter-dump`.

Example:

```bash
QEMU_PCAP=/tmp/qemu-net.pcap \
KERNEL_IMAGE=/tmp/qemu-vmlinuz \
QEMU_HOST_PORT=18080 \
make smoke

tshark -r /tmp/qemu-net.pcap -Y 'bootp || dhcp'
```

This is the easiest way to inspect guest DHCP without requiring a special host bridge.

## Environment Variables And Their Meaning

### Runtime variables

From [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go#L30) and [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go#L88):

- `GO_INIT_HTTP_ADDR`
  - override listen address; default `:8080`
- `GO_INIT_NETWORK_INTERFACE`
  - force a specific NIC such as `eth0`
- `GO_INIT_DHCP_TIMEOUT`
  - DHCP timeout duration
- `GO_INIT_DHCP_RETRY`
  - DHCP retry count
- `GO_INIT_ENABLE_DHCP`
  - disable userspace DHCP entirely if needed
- `GO_INIT_ENABLE_QEMU_USERNET_FALLBACK`
  - allow or disable the static fallback path

### QEMU variables

From [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile#L3) and [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh#L6):

- `KERNEL_IMAGE`
- `QEMU_BIN`
- `QEMU_HOST_PORT`
- `QEMU_GUEST_PORT`
- `QEMU_MEMORY`
- `QEMU_APPEND`
- `QEMU_ENABLE_VIRTIO_RNG`
- `QEMU_RNG_OBJECT`
- `QEMU_RNG_DEVICE`
- `QEMU_REQUIRE_VIRTIO_RNG`
- `QEMU_PCAP`

### Initramfs build variables

From [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile#L12):

- `INITRAMFS_ENABLE_VIRTIO_RNG_MODULE`
- `INITRAMFS_VIRTIO_RNG_MODULE_SRC`

These are important when the kernel image and available module tree do not live in their usual places.

## Extension Guide

This section is intentionally practical. It answers “where do I change the system?” rather than only “what is the system?”

### Extension 1: Add a new boot-time subsystem

Example goal:

- mount another filesystem,
- initialize another device,
- or perform another early-boot probe.

Where to put it:

- If it is purely boot scaffolding, start in [internal/boot/boot.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/boot/boot.go).
- If it is a distinct concern with structured results, create a new package like `internal/<subsystem>`.
- Wire it in [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go#L19).

Rule of thumb:

- Keep `main()` mostly about orchestration order.
- Put actual logic in internal packages.

Suggested pseudocode:

```go
results := boot.PrepareFilesystem(logger)
subsystemResult, err := subsystem.Initialize(logger)
if err != nil {
    logger.Printf("fatal: initialize subsystem: %v", err)
    boot.Halt(logger)
}
```

### Extension 2: Add a new status section to the UI

Steps:

1. Define a result struct in the relevant package.
2. Produce that result in `cmd/init`.
3. Extend `webui.Options`.
4. Extend `statusResponse`.
5. Render it in the static HTML/JS.

Files to touch:

- [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
- [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
- [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html)

### Extension 3: Change guest networking behavior

Start in [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go#L98).

Safe modifications:

- interface selection policy,
- DHCP timeout/retry defaults,
- resolver-writing behavior,
- fallback enablement.

Higher-risk modifications:

- DHCP message construction,
- raw socket behavior,
- route installation semantics.

If you change those higher-risk parts, rerun both unit tests and full QEMU smoke validation.

### Extension 4: Add another kernel module

The current system already provides the pattern.

Build-side work:

- extend [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go) to package another module file.

Boot-side work:

- add a loader function in [internal/kmod/kmod.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/kmod/kmod.go) or a sibling package.

Important invariant:

- the module bytes in the initramfs must match the kernel being booted.

### Extension 5: Replace or expand the webpage

The UI is currently a single embedded HTML file. That is a feature, not a bug.

Advantages:

- no separate frontend build toolchain,
- trivial embedding,
- easy review.

Tradeoff:

- for larger UI changes, the single-file approach will eventually become awkward.

If the project later needs a richer frontend, a reasonable next step would be a generated static asset build that is still embedded into the Go binary. For now, keeping the UI simple is the correct choice.

## Testing Strategy

There are two testing layers.

### Unit tests

The repo already has focused unit coverage for the tricky low-level pieces:

- [internal/initramfs/writer_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/initramfs/writer_test.go)
- [internal/networking/network_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network_test.go)
- [internal/entropy/entropy_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy_test.go)
- [internal/kmod/kmod_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/kmod/kmod_test.go)
- [cmd/mkinitramfs/main_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main_test.go)

Use unit tests to check:

- archive format logic,
- parsing logic,
- module-loader semantics,
- deterministic helper behavior.

### End-to-end smoke test

Use [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh#L87) to prove the whole system still boots and is reachable from the host.

This is the real contract test for the project.

## Common Failure Modes

### Failure Mode 1: QEMU cannot read the kernel image

Symptoms:

- `make run` or `make smoke` fails before guest boot.

Cause:

- host kernel image under `/boot` is not readable by the current user.

Fix:

- copy the kernel to a readable path and pass `KERNEL_IMAGE=/tmp/qemu-vmlinuz`.

### Failure Mode 2: HTTP never becomes reachable

Symptoms:

- host `curl` hangs or times out.

Likely causes:

- guest networking failed,
- QEMU port forwarding is wrong,
- guest crashed before HTTP server start.

Debug path:

1. inspect `build/qemu-smoke.log`
2. inspect `/api/status` if reachable
3. use `QEMU_PCAP=/tmp/qemu-net.pcap`

### Failure Mode 3: `virtio-rng` device exists but is not visible in the guest

Symptoms:

- `virtioRngVisible` is `false`
- `rngCurrent` is `none`

Likely causes:

- `QEMU_ENABLE_VIRTIO_RNG=0`
- module not packaged
- module does not match kernel
- module load failed

### Failure Mode 4: Module load returns `exec format error`

This happened during development and is now explicitly guarded against.

Cause:

- compressed `.ko.zst` bytes were passed directly to `finit_module` instead of packaging an ELF `.ko`.

Fix:

- keep the decompression path in [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go#L105).

## Design Decisions

### Decision 1: Keep the runtime in Go, not shell

Rationale:

- easier testing,
- stronger structure and typed results,
- fewer hidden dependencies on tools inside the initramfs.

### Decision 2: Keep the initramfs builder in-repo and rootless

Rationale:

- reproducible from user space,
- no dependence on host `cpio` or privileged `mknod` workflows beyond the builder’s own archive output.

### Decision 3: Keep the UI embedded and simple

Rationale:

- avoids a second build toolchain,
- appropriate for a status/reporting surface,
- keeps “single binary” claim intact.

### Decision 4: Prefer explicit observability over silent magic

Rationale:

- status JSON includes mounts, networking, entropy, and module state,
- smoke asserts real outcomes,
- logs are intended to be readable during debugging.

## Alternatives Considered

### Use a traditional init system

Rejected because:

- it defeats the point of the demo,
- it hides the early-boot logic the repo is meant to teach.

### Depend on kernel `ip=dhcp`

Rejected because:

- host kernels may not have the necessary config,
- userspace DHCP gives the repo more deterministic behavior.

### Build a custom kernel with built-in `virtio_rng`

Deferred because:

- loading the matching module from the initramfs solved the problem with less infrastructure.

### Use a multi-page or framework-driven frontend

Rejected for current scope because:

- the UI is mostly a renderer for a single status document,
- the extra toolchain cost is not justified yet.

## Implementation Plan For Future Maintainers

When you add a feature, follow this order:

1. Identify whether it is build-time, boot-time, runtime, or UI work.
2. Add or update unit tests for the smallest tricky logic first.
3. Wire the feature into `cmd/init/main.go` only after its package logic is understandable in isolation.
4. Update `/api/status` if an operator needs to see it.
5. Update smoke validation if the feature is a boot invariant.
6. Update ticket docs and upload the new bundle if the change is substantial.

This order keeps the repository reviewable.

## Open Questions

- Should `/healthz` remain a pure HTTP liveness check, or should it eventually fail on missing critical subsystems such as networking or entropy?
- Should the repo eventually persist a random seed across boots once writable guest storage exists?
- Should the UI remain a single HTML file if the project grows beyond status reporting?

## References

- [README.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/README.md)
- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
- [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
- [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go)
- [internal/boot/boot.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/boot/boot.go)
- [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go)
- [internal/kmod/kmod.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/kmod/kmod.go)
- [internal/entropy/entropy.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy.go)
- [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
- [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html)
- [internal/initramfs/writer.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/initramfs/writer.go)
- [01-early-boot-entropy-support-architecture-and-implementation-guide-for-the-qemu-go-init-runtime.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/design-doc/01-early-boot-entropy-support-architecture-and-implementation-guide-for-the-qemu-go-init-runtime.md)
- [02-implementation-guide.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/reference/02-implementation-guide.md)
- [01-diary.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/reference/01-diary.md)
