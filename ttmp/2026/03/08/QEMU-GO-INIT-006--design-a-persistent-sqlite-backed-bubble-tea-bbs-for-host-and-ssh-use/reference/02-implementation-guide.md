---
Title: Implementation guide
Ticket: QEMU-GO-INIT-006
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - ssh
    - tui
    - sqlite
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Copy-paste operational guide for implementing and validating the shared-state Bubble Tea BBS.
LastUpdated: 2026-03-08T19:08:00-04:00
WhatFor: Copy-paste operational guide for implementing and validating the shared-state Bubble Tea BBS.
WhenToUse: Use this while actively implementing ticket 006 or when reproducing its experiments.
---

# Implementation guide

## Goal

This document gives the concrete commands, package layout, API sketches, and validation steps for ticket `QEMU-GO-INIT-006`.

## Context

The current repo already provides:

- a Go PID 1 guest runtime
- a QEMU smoke test path
- block-backed guest persistence
- a Wish SSH server

Ticket `006` adds:

- a shared host directory mounted into the guest with `9p`
- a SQLite BBS database in that shared directory
- a Bubble Tea application package reused by host and guest entrypoints

## Quick Reference

### Shared-state paths

- host path: `build/shared-state`
- guest mount tag: `hostshare`
- guest mount point: `/var/lib/go-init/shared`
- database path: `bbs.db` under the chosen state root

### Recommended package layout

```text
cmd/bbs
internal/bbsapp
internal/bbsstore
internal/sharedstate
internal/sshbbs
```

### Store interface sketch

```go
type Message struct {
    ID        int64
    Author    string
    Subject   string
    Body      string
    CreatedAt time.Time
}

type Store interface {
    ListMessages(ctx context.Context) ([]Message, error)
    GetMessage(ctx context.Context, id int64) (Message, error)
    CreateMessage(ctx context.Context, params CreateMessageParams) (Message, error)
}
```

### Host CLI loop

```text
resolve state root
open store
construct bbs model
tea.NewProgram(model, tea.WithAltScreen())
run
```

### Guest boot flow

```text
prepare local storage
load virtio_rng
prepare shared 9p mount
configure networking
start ssh service with Bubble Tea middleware
start HTTP diagnostics page
```

### QEMU shape

```text
-drive file=build/data.img,if=virtio,format=raw
-virtfs local,path=build/shared-state,mount_tag=hostshare,security_model=none,id=hostshare
-device virtio-9p-pci,fsdev=hostshare,mount_tag=hostshare
```

## Usage Examples

### Probe the library stack

```bash
cd ttmp/2026/03/08/QEMU-GO-INIT-006--design-a-persistent-sqlite-backed-bubble-tea-bbs-for-host-and-ssh-use/scripts/bbs-stack-probe
go mod tidy
go run .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/bbs-stack-probe-linux .
```

Expected outcome:

- SQLite opens and writes
- Lip Gloss renderer constructs successfully
- Bubble Tea program constructs successfully

### Check kernel support for the shared mount path

```bash
rg -n 'CONFIG_(NET_9P|9P_FS|VIRTIO_FS)=' /boot/config-$(uname -r)
find /lib/modules/$(uname -r) -type f | rg '/(9p|9pnet|9pnet_virtio)\\.ko(\\.zst)?$'
```

### Host-native BBS

Target command after implementation:

```bash
go run ./cmd/bbs
```

### Guest BBS

Target command after implementation:

```bash
make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 QEMU_SSH_HOST_PORT=10022
ssh -p 10022 127.0.0.1
```

## Related

- [design-doc/01-sqlite-backed-bubble-tea-bbs-architecture-analysis-and-implementation-guide-for-host-and-guest-runtimes.md](../design-doc/01-sqlite-backed-bubble-tea-bbs-architecture-analysis-and-implementation-guide-for-host-and-guest-runtimes.md)
- [reference/01-diary.md](./01-diary.md)
