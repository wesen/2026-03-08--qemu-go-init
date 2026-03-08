---
Title: Early-boot entropy support architecture and implementation guide for the QEMU Go init runtime
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
      Note: QEMU run path that needs virtio-rng support
    - Path: cmd/init/main.go
      Note: PID 1 entrypoint where entropy diagnostics will be gathered
    - Path: internal/webui/site.go
      Note: Status API surface that will expose entropy diagnostics
    - Path: scripts/qemu-smoke.sh
      Note: Smoke path that needs virtio-rng and logging support
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T16:35:03.285531739-04:00
WhatFor: ""
WhenToUse: ""
---


# Early-boot entropy support architecture and implementation guide for the QEMU Go init runtime

## Executive Summary

This ticket adds explicit early-boot entropy support to the QEMU demo environment and exposes the guest's entropy state through the running Go PID 1 runtime. The immediate goal is not full seed lifecycle management. It is to make entropy a first-class, visible system dependency by adding a virtual RNG device to the standard QEMU flow and surfacing the guest's observed entropy state in `/api/status` and the embedded webpage.

The design is intentionally narrow. The repo already has a working boot path, a working DHCP client, and a status API. The next step is to add QEMU `virtio-rng` support, probe kernel-reported entropy state from the guest, and make that status visible enough that a human operator can tell whether the guest is booting with a credible randomness source.

## Problem Statement

The previous DHCP work exposed a real systems risk: early-boot code can block on randomness long before the failure is obvious from the outside. That incident was solved tactically by avoiding an upstream helper that hid a blocking random-number call, but the environment still lacks an explicit entropy strategy.

Current gaps:

- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile) does not launch QEMU with a virtual RNG device.
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh) does not include `virtio-rng` in the smoke environment.
- [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go) does not probe entropy state.
- [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go) and [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html) do not expose entropy diagnostics.

Without those pieces, the demo is still vulnerable to hidden early-boot randomness assumptions, and the operator has no built-in visibility into whether the guest sees a usable RNG path.

After the first implementation slices in this ticket, there is now a narrower remaining gap:

- QEMU launches with `virtio-rng`, and the guest reports entropy diagnostics,
- but the current Ubuntu host kernel used for the guest boot has `CONFIG_HW_RANDOM_VIRTIO=m`,
- the initramfs does not ship kernel modules,
- so the guest cannot currently activate the `virtio_rng` driver even though the device is present in QEMU.

## Proposed Solution

Implement entropy support in three coordinated layers.

### 1. QEMU launch support

Extend the repo's QEMU run paths so the guest sees a `virtio-rng` device. The standard shape is:

```text
-object rng-random,id=rng0,filename=/dev/urandom
-device virtio-rng-pci,rng=rng0
```

This belongs in:

- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)

The flags should remain configurable via environment variables so the operator can disable or customize the entropy backend if needed.

### 2. Guest entropy diagnostics package

Add a new `internal/entropy` package that performs read-only probing of kernel and device state. It should not fail the boot if optional files are absent. It should return a stable result struct containing fields such as:

- kernel-reported entropy availability,
- whether `/dev/hwrng` exists,
- whether `rng_available` or `rng_current` report a backend,
- any probe warnings or missing-file conditions that matter for debugging.

Candidate probe paths:

- `/proc/sys/kernel/random/entropy_avail`
- `/sys/class/misc/hw_random/rng_available`
- `/sys/class/misc/hw_random/rng_current`
- `/dev/hwrng`

### 3. UI and API exposure

Plumb the entropy result into the existing handler options and expose it in `/api/status`. The embedded webpage should show enough information for a human to answer:

- Is a hardware or virtual RNG visible?
- What entropy value is the kernel reporting?
- Which backend, if any, is active?
- Are there obvious warnings about missing entropy infrastructure?

## Design Decisions

### Decision 1: Make entropy support part of the default QEMU demo path

Rationale:

- This repo is a boot demo.
- The default environment should include the infrastructure that makes early boot less fragile.

### Decision 2: Keep diagnostics read-only in this ticket

Rationale:

- Seed persistence and secret lifecycle policy are larger topics.
- This ticket should improve the boot environment and observability without widening into storage design.

### Decision 3: Degrade gracefully when kernel files are absent

Rationale:

- Minimal kernels vary in what they expose.
- Diagnostics should prefer partial visibility over boot failure.

## Alternatives Considered

### Only document the entropy requirement

Rejected because:

- documentation alone does not make the repo safer,
- operators still would not see the entropy state from the running guest.

### Add full persistent seed management now

Rejected because:

- it introduces storage and lifecycle policy questions,
- it is not required to make the current demo environment materially better.

### Leave entropy handling entirely to callers

Rejected because:

- the standard smoke path is the primary operational entrypoint for this repo,
- it should model the intended system environment instead of remaining under-specified.

## Implementation Plan

### Phase 1: Ticket scaffold

- Create the ticket, docs, and tasks.
- Capture the architecture and code surfaces.

### Phase 2: QEMU `virtio-rng` plumbing

- Update [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile).
- Update [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh).
- Ensure the launch log includes whether entropy support is enabled.

### Phase 3: Guest diagnostics

- Add `internal/entropy`.
- Probe procfs/sysfs and device existence.
- Add focused unit tests for parsing and probe behavior.

### Phase 4: Web/API integration

- Update [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go).
- Update [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go).
- Update [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html).

### Phase 5: Validation

- Run `make test`.
- Run `make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080`.
- Inspect `/api/status`.
- Record results in the diary and changelog.

### Phase 6: Kernel-side follow-up

Choose one of:

- boot a kernel with `CONFIG_HW_RANDOM_VIRTIO=y`,
- or extend the initramfs/runtime to ship and load the `virtio_rng` module for the booted kernel.

## Open Questions

Open questions are intentionally limited:

- Should `virtio-rng` be enabled by default in every QEMU path or only smoke/debug by default?
- Should low entropy ever affect health reporting, or remain informational in this ticket?
- Should the repo solve kernel-side `virtio_rng` activation here or in a follow-up ticket once the module-loading/custom-kernel path is chosen?
- Should a follow-up ticket add seed persistence once the project has a writable guest storage model?

## References

- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
- [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
- [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
- [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go)
