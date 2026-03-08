---
Title: Single-binary Go init architecture and implementation guide
Ticket: QEMU-GO-INIT-001
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - web
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: Makefile
      Note: Build_and_QEMU_workflow
    - Path: cmd/init/main.go
      Note: Guest_PID_1_entrypoint
    - Path: cmd/mkinitramfs/main.go
      Note: Rootless_initramfs_builder
    - Path: internal/boot/boot.go
      Note: Mounting_and_signal_handling
    - Path: internal/initramfs/writer.go
      Note: newc_archive_writer
    - Path: internal/webui/site.go
      Note: Embedded_web_UI_and_status_API
    - Path: internal/webui/static/index.html
      Note: Browser_dashboard_asset
    - Path: scripts/qemu-smoke.sh
      Note: Automated_QEMU_smoke_flow
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T14:14:54.120745419-04:00
WhatFor: ""
WhenToUse: ""
---


# Single-binary Go init architecture and implementation guide

## Executive Summary

This repository now implements the smallest practical version of the imported QEMU guide: a statically linked Go binary becomes `/init`, mounts the minimum early-boot filesystems, serves an embedded webpage on port `8080`, and is packed into an initramfs that QEMU can boot with `-kernel` and `-initrd`.

The imported source document in `sources/local/qemu-go-guide.md` argued for an external initramfs first, then a later move to `CONFIG_INITRAMFS_SOURCE`. The implementation follows that shape, but improves the build ergonomics by replacing `sudo mknod` and shell `cpio` assembly with a rootless Go initramfs writer. That keeps the proof of concept inside the Go module, makes the artifact pipeline reproducible, and removes a privileged setup step for local iteration.

The result is intentionally narrow in scope. It is not a general-purpose distro, init system, or container host. It is a focused teaching and validation project that demonstrates four core ideas:

1. Linux will execute `/init` from an initramfs as PID 1.
2. A single statically linked Go binary can play that role.
3. QEMU user networking can forward a host TCP port into the guest.
4. Embedded assets let the Go binary serve a human-readable webpage without additional files in the guest.

## Problem Statement

The user request was to create a new `docmgr` ticket, import `/tmp/qemu-go-guide.md`, read it, implement what it recommended, and produce a detailed design and implementation guide suitable for a new intern. The repository started empty except for `.git`, so both the code and the project documentation needed to be created from scratch.

The imported guide proposed the following proof-of-concept architecture:

- Build one statically linked Go binary.
- Place that binary at `/init` inside a tiny initramfs.
- Boot a Linux kernel in QEMU with that initramfs.
- Use QEMU user networking plus `hostfwd` so the host can reach the guest HTTP server.

That proposal is a good fit for a small, understandable prototype, but it still left several engineering questions open:

- How should PID 1 handle signals and child reaping so the example remains valid as an init process?
- How can the initramfs be built without requiring `sudo mknod` or a hand-written shell archive step?
- How should the webpage and API be structured so the result is more useful than a plain `"hello world"` response?
- How can the project expose a stable local developer workflow through `make` and a smoke script?
- How should the documentation explain Linux boot concepts clearly enough for an intern who may never have worked with QEMU, initramfs, or PID 1 before?

The implementation answers those questions while keeping the system deliberately small.

## Scope

In scope:

- A statically linked Linux Go binary for `/init`.
- A small embedded webpage and JSON status API served from the guest.
- A rootless `newc` initramfs writer implemented in Go.
- `make` targets for building, packaging, running, and smoke testing.
- Ticket documentation, diary, and reMarkable delivery.

Out of scope:

- Building a custom kernel in this repository.
- Embedding the initramfs into the kernel image.
- Running user workloads beyond the demo HTTP server.
- Full init semantics such as service supervision, poweroff, or shutdown orchestration.

## Current-State Architecture

The repository now has six core areas, each with a narrow job:

1. `cmd/init/main.go`
   This is the guest entrypoint. It sets up logging, starts PID 1 signal handling, mounts early filesystems, builds the web handler, and blocks forever in the HTTP server. The entire boot sequence is visible in about twenty lines, which makes it easy for new engineers to trace the happy path. See `cmd/init/main.go:11-31`.

2. `internal/boot/boot.go`
   This package contains the PID 1 runtime concerns: filesystem mount attempts, HTTP address selection, SIGCHLD reaping, request logging, and a halt loop used when startup fatally fails. The mount table is explicit in `mountSpecs`, and the signal policy is explicit in `StartChildReaper`. See `internal/boot/boot.go:30-137`.

3. `internal/webui/site.go` and `internal/webui/static/index.html`
   This package embeds the frontend assets, publishes `/healthz`, and publishes `/api/status`. The JSON response is intentionally small but useful: PID, hostname, Go runtime version, listen address, uptime, and mount outcomes. See `internal/webui/site.go:15-73`.

4. `cmd/mkinitramfs/main.go`
   This is the build-side tool that turns the compiled init binary into `build/initramfs.cpio.gz`. It reads the binary, creates a gzip stream, and writes the minimal filesystem layout needed for the guest: `dev`, `proc`, `sys`, `/dev/console`, `/dev/null`, and `/init`. See `cmd/mkinitramfs/main.go:29-99`.

5. `internal/initramfs/writer.go`
   This package writes `newc` cpio entries directly. It supports directory entries, regular files, character devices, and the trailing `TRAILER!!!` record. The implementation is small enough for an intern to read in one sitting, but specific enough to eliminate the need for root privileges during archive creation. See `internal/initramfs/writer.go:15-154`.

6. `Makefile` and `scripts/qemu-smoke.sh`
   These files define the developer workflow. `make build` creates the statically linked `/init` binary, `make initramfs` packages it, `make run` boots QEMU interactively, and `make smoke` attempts an automated boot plus HTTP verification. See `Makefile:1-43` and `scripts/qemu-smoke.sh:1-59`.

## Imported Guide to Repository Mapping

The imported guide in `ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/sources/local/qemu-go-guide.md` recommended the following flow:

1. Build a static Go `/init`.
2. Build a tiny initramfs.
3. Boot it with QEMU using host networking port forwarding.
4. Fall back to a custom kernel if host-kernel boot fails.
5. Later embed the initramfs into the kernel.

The repository implements items 1 through 3 directly and documents item 4 as the next move when a readable kernel image is unavailable. It intentionally stops short of item 5 because that requires kernel build configuration and would make the proof of concept materially larger.

The main design change relative to the guide is build tooling:

- The guide used shell `cpio` plus privileged `mknod`.
- The repository uses `cmd/mkinitramfs` plus `internal/initramfs.Writer`.
- The guide assumed `/boot/vmlinuz-*` would be directly usable.
- The repository now detects when no readable kernel image is present and surfaces that failure early in `make run` and `scripts/qemu-smoke.sh`.

## Proposed Architecture

### Runtime View

```text
Host shell / browser
        |
        | make run / make smoke
        v
QEMU process on host
        |
        | -kernel <readable kernel image>
        | -initrd build/initramfs.cpio.gz
        | -nic user,hostfwd=tcp::8080-:8080
        v
Linux guest kernel
        |
        | unpacks initramfs into rootfs
        | executes /init as PID 1
        v
Go /init binary
        |
        | mounts /proc, /sys, /dev
        | starts HTTP server on :8080
        v
Embedded web UI + /api/status
```

### Build View

```text
Go source
  |
  | go build ./cmd/init
  v
build/init  (static ELF)
  |
  | go run ./cmd/mkinitramfs -init-bin build/init -output build/initramfs.cpio.gz
  v
build/initramfs.cpio.gz
  |
  | qemu-system-x86_64 -kernel ... -initrd ...
  v
Booted guest serving host-forwarded HTTP traffic
```

## Component Deep Dive

### 1. `cmd/init`: the guest's PID 1 entrypoint

`main()` is intentionally short because the startup path should be obvious. The sequence is:

1. Create a UTC logger.
2. Start the signal/reaper loop.
3. Mount `proc`, `sysfs`, and `devtmpfs`.
4. Resolve the listen address from `GO_INIT_HTTP_ADDR` or default to `:8080`.
5. Build the web handler with mount results attached.
6. Start the HTTP server.
7. If anything fatal happens, enter a halt loop instead of returning.

That last point matters because PID 1 is special. Returning from `/init` is not a normal application exit; it usually collapses the boot process and can trigger a kernel panic. The `boot.Halt()` path in `internal/boot/boot.go:83-90` makes the failure mode explicit and avoids silently leaving PID 1.

Pseudo-flow:

```text
logger := newLogger()
StartChildReaper(logger)
mounts := PrepareFilesystem(logger)
addr := HTTPAddress()
handler := webui.NewHandler(addr, mounts)
if handler setup fails:
    Halt()
ServeHTTP(addr, handler)
if server exits unexpectedly:
    Halt()
```

### 2. `internal/boot`: Linux boot responsibilities that do not belong in the UI

The `internal/boot` package exists to keep Linux- and PID-1-specific behavior separate from the HTTP handlers.

Important responsibilities:

- `PrepareFilesystem()` iterates over a fixed mount table and returns structured results rather than only log strings.
- `HTTPAddress()` gives one configuration point through `GO_INIT_HTTP_ADDR`.
- `StartChildReaper()` listens for `SIGCHLD`, `SIGINT`, and `SIGTERM`.
- `reapChildren()` loops on `wait4(..., WNOHANG, ...)` until no more exited children remain.
- `ServeHTTP()` wraps the handler with request logging before calling `ListenAndServe`.

Why this matters:

- PID 1 often behaves differently from ordinary processes for signal handling and zombie reaping.
- Returning mount results as data makes the webpage and JSON endpoint educational; the page can show what was attempted and whether it worked.
- Keeping this code out of `cmd/init` prevents the entrypoint from turning into an unreadable ball of syscalls.

### 3. `internal/webui`: an embedded frontend instead of a literal string response

The imported guide used a plain text handler. That is fine for the smallest hello-world test, but the user explicitly asked for a webpage and an intern-facing explanation. The repository therefore embeds `static/index.html` using `go:embed` and serves it with `http.FileServer`.

The handler publishes:

- `/`
  The HTML dashboard with a styled explanation of the architecture and live boot status.
- `/healthz`
  A minimal liveness endpoint used by the smoke script.
- `/api/status`
  A JSON endpoint that the page polls every five seconds.

The `statusResponse` shape in `internal/webui/site.go:23-33` is the user-facing contract. It is intentionally tiny but sufficient for debugging:

- `pid`
- `hostname`
- `goVersion`
- `goos`
- `goarch`
- `listenAddr`
- `startedAt`
- `uptime`
- `mounts`

This arrangement gives both machine-readable and human-readable inspection paths without adding third-party dependencies.

### 4. `cmd/mkinitramfs`: rootless archive assembly

The imported guide's shell recipe is accurate, but it requires:

- directory staging on disk,
- device nodes created in that tree,
- `cpio`,
- `gzip`,
- and in most environments, elevated privileges for `mknod`.

The repository replaces that with a Go tool that writes the archive directly. `cmd/mkinitramfs/main.go:64-99` shows the complete manifest. That manifest is important because it explains exactly what the guest filesystem contains before PID 1 runs:

- `dev/`
- `proc/`
- `sys/`
- `dev/console`
- `dev/null`
- `init`

This is not just a convenience layer. It materially improves reproducibility because the archive content is now code-reviewed Go rather than an implicit shell pipeline.

### 5. `internal/initramfs.Writer`: `newc` cpio as a library

The initramfs writer is the most low-level component in the repository. It:

- formats `newc` headers as fixed-width hexadecimal strings,
- writes names terminated by a NUL byte,
- pads names and data to four-byte alignment,
- supports character device metadata,
- appends the `TRAILER!!!` sentinel on close.

That is enough to generate a valid compressed archive for QEMU boot while still being understandable for new engineers. The unit test in `internal/initramfs/writer_test.go` parses the produced archive and checks the directory entry, the character-device entry, the regular file entry, and the trailer. That keeps the format logic from becoming a fragile black box.

### 6. Automation: `Makefile` and `scripts/qemu-smoke.sh`

The repository uses a deliberately small automation layer:

- `make build`
  Compiles the static Linux binary.
- `make initramfs`
  Builds `build/initramfs.cpio.gz`.
- `make run`
  Boots QEMU interactively with host port forwarding.
- `make smoke`
  Boots QEMU in the background, waits for `/healthz`, requests `/` and `/api/status`, and exits.

The smoke script also encodes an important environmental rule: a kernel image must be readable by the current user. The current host stores `/boot/vmlinuz-*` with mode `0600`, so the repository now fails early with a clear message rather than letting QEMU fail later with a less contextualized boot error.

## API Reference

### Go entrypoints

| API | Purpose | Notes |
| --- | --- | --- |
| `main.main` in `cmd/init` | Boot the guest process as PID 1 | Keeps the startup path compact |
| `boot.PrepareFilesystem` | Mount early filesystems and collect results | Returns data for logs and UI |
| `boot.StartChildReaper` | Handle PID 1 signal responsibilities | Reaps zombies on `SIGCHLD` |
| `boot.HTTPAddress` | Resolve listen address | Reads `GO_INIT_HTTP_ADDR` |
| `webui.NewHandler` | Build the embedded site and API | Exposes `/`, `/healthz`, `/api/status` |
| `initramfs.NewWriter` | Create a `newc` archive writer | Used only by build tooling |
| `Writer.AddDirectory` | Add directory entries | Used for `dev`, `proc`, `sys` |
| `Writer.AddCharDevice` | Add device nodes | Used for `/dev/console` and `/dev/null` |
| `Writer.AddFile` | Add regular file entries | Used for `/init` |

### External command surface

| Command | Purpose |
| --- | --- |
| `make build` | Produce `build/init` |
| `make initramfs` | Produce `build/initramfs.cpio.gz` |
| `make run` | Start QEMU with interactive console output |
| `make smoke QEMU_HOST_PORT=18080` | Attempt automated guest verification |
| `KERNEL_IMAGE=/path/to/bzImage make run` | Override kernel path when `/boot` is unreadable |

## Detailed Boot Sequence

### Step-by-step narrative

1. The host developer runs `make run`.
2. `Makefile` builds `build/init` with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`.
3. `cmd/mkinitramfs` writes the compressed initramfs archive.
4. QEMU boots the chosen kernel with `-initrd build/initramfs.cpio.gz`.
5. The Linux kernel unpacks the archive into `rootfs`.
6. The kernel executes `/init`.
7. The Go binary becomes PID 1 and initializes mount points.
8. The HTTP server listens on `:8080`.
9. QEMU user networking forwards host port `8080` into guest port `8080`.
10. The host browser or `curl` reaches the demo page and JSON status endpoint.

### Pseudocode for the end-to-end flow

```text
host:
  make run
  -> go build ./cmd/init
  -> go run ./cmd/mkinitramfs
  -> qemu-system-x86_64 -kernel KERNEL_IMAGE -initrd build/initramfs.cpio.gz

guest kernel:
  unpack initramfs
  exec /init

guest userspace (/init):
  start logger
  start child reaper
  for each mount in [proc, sysfs, devtmpfs]:
      mkdir target
      mount(source, target, fstype)
  mux := build web handler with boot metadata
  listen on :8080 forever

host networking:
  hostfwd tcp host:8080 -> guest:8080
  curl /healthz
  curl /api/status
```

## Design Decisions and Tradeoffs

### Decision: keep the guest userspace as one Go binary

Reasoning:

- It matches the imported guide exactly.
- It makes the artifact boundary easy to explain.
- It keeps dependency count at zero beyond the Go standard library.

Tradeoff:

- The example does not show multi-process orchestration or supervision.

### Decision: implement the initramfs builder in Go instead of shell

Reasoning:

- Avoids privileged `mknod`.
- Avoids depending on the host's `cpio` invocation details.
- Makes the filesystem manifest reviewable as source code.

Tradeoff:

- The repository now owns low-level `newc` format code and corresponding tests.

### Decision: serve a webpage and JSON API instead of a single text string

Reasoning:

- The user explicitly requested a webpage.
- The JSON endpoint improves debuggability.
- The HTML page doubles as living documentation when the VM is running.

Tradeoff:

- Slightly more moving parts than a single handler function.

### Decision: detect unreadable kernel images early

Reasoning:

- This host keeps `/boot/vmlinuz-*` root-only.
- Failing with a direct message is easier to debug than letting QEMU report a generic launch failure.

Tradeoff:

- `make run` still depends on an external kernel image.
- Truly single-artifact boot remains a future kernel-embedding step, not the current state.

## Validation Strategy and Results

Completed validation:

- `make test`
  Passed. This verifies the initramfs writer and confirms the module compiles across all packages.
- `make initramfs`
  Passed. This produced `build/init` and `build/initramfs.cpio.gz`.
- `file build/init build/initramfs.cpio.gz`
  Confirmed the binary is statically linked and the archive is gzip-compressed.

Blocked validation:

- `make smoke QEMU_HOST_PORT=18080`
  The first attempt failed after QEMU reported `could not open kernel file '/boot/vmlinuz-6.8.0-90-generic': Permission denied`.
- After tightening kernel detection, the next attempt failed earlier and more clearly because there was no readable `/boot/vmlinuz-*` image for the current user.

Interpretation:

- The repository implementation is buildable and testable.
- The missing piece on this host is not the initramfs or the Go binary; it is access to a bootable, readable kernel image.

## Risks, Alternatives, and Open Questions

### Risks

- PID 1 behavior remains intentionally minimal. If future versions spawn subprocesses, shutdown and lifecycle semantics will need deeper treatment.
- The current runtime depends on `devtmpfs` support in the chosen kernel.
- QEMU networking still assumes a kernel with usable built-in networking for the chosen boot path.

### Alternatives considered

1. Use the imported shell `cpio` recipe directly.
   Rejected because it would likely require `sudo mknod` and would be less reproducible.
2. Build a custom kernel immediately.
   Rejected for the first pass because it would expand the repo and the intern cognitive load significantly.
3. Serve plain text only.
   Rejected because the user requested a webpage and detailed educational material.

### Open questions

1. Should the next iteration build a small custom kernel or document how to obtain a readable host kernel artifact?
2. Should a later revision embed the initramfs into the kernel using `CONFIG_INITRAMFS_SOURCE` to approach a single boot artifact?
3. Should the runtime grow a structured shutdown path rather than halting indefinitely on fatal startup errors?

## References

### Source files

- `cmd/init/main.go:11-31`
- `internal/boot/boot.go:30-137`
- `internal/webui/site.go:15-73`
- `internal/webui/static/index.html`
- `cmd/mkinitramfs/main.go:29-99`
- `internal/initramfs/writer.go:15-154`
- `internal/initramfs/writer_test.go`
- `Makefile:1-43`
- `scripts/qemu-smoke.sh:1-59`
- `README.md`

### Ticket sources

- `sources/local/qemu-go-guide.md`

### Useful validation commands

```bash
make test
make initramfs
file build/init build/initramfs.cpio.gz
KERNEL_IMAGE=/path/to/readable/bzImage make run
make smoke QEMU_HOST_PORT=18080 KERNEL_IMAGE=/path/to/readable/bzImage
```

## Proposed Solution

<!-- Describe the proposed solution in detail -->

## Design Decisions

<!-- Document key design decisions and rationale -->

## Alternatives Considered

<!-- List alternative approaches that were considered and why they were rejected -->

## Implementation Plan

<!-- Outline the steps to implement this design -->

## Open Questions

<!-- List any unresolved questions or concerns -->

## References

<!-- Link to related documents, RFCs, or external resources -->
