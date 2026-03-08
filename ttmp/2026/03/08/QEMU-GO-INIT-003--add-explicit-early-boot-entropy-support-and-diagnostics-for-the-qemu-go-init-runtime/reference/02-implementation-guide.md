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
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T16:35:36.322849851-04:00
WhatFor: ""
WhenToUse: ""
---

# Implementation guide

## Goal

Provide a concrete, file-level implementation checklist for explicit early-boot entropy support in the repo.

## Context

The repo already has working QEMU boot automation, a Go PID 1 runtime, and a status page. This ticket adds entropy support by improving the QEMU environment and making entropy state visible inside the guest.

## Quick Reference

### Planned code surfaces

- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
- [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
- [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
- [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html)
- new package: `internal/entropy`

### Pseudocode

```go
entropyResult := entropy.Probe(logger)

handler, err := webui.NewHandler(webui.Options{
    ListenAddr: addr,
    Mounts:     results,
    Network:    networkResult,
    Entropy:    entropyResult,
})
```

```bash
QEMU_ARGS+=(
  -object rng-random,id=rng0,filename=/dev/urandom
  -device virtio-rng-pci,rng=rng0
)
```

### Validation commands

```bash
make test
make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080
curl -fsS http://127.0.0.1:18080/api/status
```

## Usage Examples

Use this guide while implementing the ticket in small commits:

1. Land QEMU launch support.
2. Land guest entropy probing.
3. Land UI/API exposure.
4. Validate and document.

## Related

- [01-early-boot-entropy-support-architecture-and-implementation-guide-for-the-qemu-go-init-runtime.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/design-doc/01-early-boot-entropy-support-architecture-and-implementation-guide-for-the-qemu-go-init-runtime.md)
- [01-diary.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/reference/01-diary.md)
