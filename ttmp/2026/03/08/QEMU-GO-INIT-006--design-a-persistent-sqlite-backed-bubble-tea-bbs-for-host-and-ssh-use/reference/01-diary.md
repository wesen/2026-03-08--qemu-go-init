---
Title: Diary
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
Summary: Chronological implementation diary for ticket 006, including experiments, decisions, and follow-up notes.
LastUpdated: 2026-03-08T19:08:00-04:00
WhatFor: Chronological implementation diary for ticket 006, including experiments, decisions, and follow-up notes.
WhenToUse: Read this when reviewing what changed, what was tested, and why the design moved in a particular direction.
---

# Diary

## Goal

Record the step-by-step work for ticket `QEMU-GO-INIT-006`.

## Context

This ticket started as a design exercise for a SQLite-backed Bubble Tea BBS reachable over SSH and reusable as a host-native binary. During design, the key persistence question became: where should the database live if both the host binary and the guest SSH app need to use it?

## Quick Reference

### Timeline

#### 2026-03-08 18:45 America/New_York

- Created ticket `QEMU-GO-INIT-006`.
- Added three starter documents:
  - design doc
  - implementation guide
  - diary

#### 2026-03-08 18:50 America/New_York

- Reviewed current runtime files:
  - [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
  - [internal/sshapp/server.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/server.go)
  - [internal/storage/storage.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/storage/storage.go)
  - [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
  - [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)

#### 2026-03-08 18:56 America/New_York

- Created a ticket-local probe module under:
  - [scripts/bbs-stack-probe/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-006--design-a-persistent-sqlite-backed-bubble-tea-bbs-for-host-and-ssh-use/scripts/bbs-stack-probe/main.go)
  - [scripts/bbs-stack-probe/go.mod](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-006--design-a-persistent-sqlite-backed-bubble-tea-bbs-for-host-and-ssh-use/scripts/bbs-stack-probe/go.mod)
- Ran:

```bash
go mod tidy
go run .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/bbs-stack-probe-linux .
```

- Result:
  - the stack worked with pure-Go SQLite
  - Bubble Tea and Lip Gloss constructed successfully

#### 2026-03-08 19:00 America/New_York

- Investigated terminal capability dependencies.
- Confirmed that the Charmbracelet stack does not require adding ncurses userland to the guest.
- Also confirmed that terminal capability logic is not "magically absent":
  - `termenv`
  - `colorprofile`
  - `xo/terminfo`

#### 2026-03-08 19:05 America/New_York

- Reframed the storage plan after clarifying the host/guest terminology with the user.
- Important conclusion:
  - guest-local raw `ext4` storage is fine for guest-only persistence
  - it is awkward for a host-native program that wants to open the same SQLite file directly
- New plan:
  - use a shared host directory for BBS content
  - pass it into the guest with QEMU

#### 2026-03-08 19:07 America/New_York

- Checked host support for the pass-through options.
- Findings:
  - `virtiofsd` is not installed on this host
  - the kernel at `/boot/config-$(uname -r)` enables `CONFIG_NET_9P=m` and `CONFIG_9P_FS=m`
  - module files exist for:
    - `9p.ko.zst`
    - `9pnet.ko.zst`
    - `9pnet_virtio.ko.zst`
- Decision:
  - implement `9p` first
  - keep `virtiofs` as the cleaner later migration

#### 2026-03-08 19:20 America/New_York

- Implemented the first code slice for the new storage plan.
- Added generic kernel-module packaging to:
  - [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go)
- Added guest shared-state mount support to:
  - [internal/sharedstate/sharedstate.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sharedstate/sharedstate.go)
- Extended the module loader with `9p` helpers in:
  - [internal/kmod/kmod.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/kmod/kmod.go)
- Wired QEMU shared-directory flags into:
  - [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
  - [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
- Exposed shared-state status from:
  - [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
  - [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
- Added tests for the new initramfs flag shape and shared-state helpers.
- Validation:

```bash
gofmt -w cmd/init/main.go cmd/mkinitramfs/main.go cmd/mkinitramfs/main_test.go internal/kmod/kmod.go internal/sharedstate/sharedstate.go internal/sharedstate/sharedstate_test.go internal/webui/site.go
go test ./...
```

- Result:
  - test suite passed
  - no runtime smoke yet because the BBS application layer is still pending

## Usage Examples

Current next steps:

1. Add the reusable SQLite store and Bubble Tea app packages.
2. Add the host `cmd/bbs` entrypoint.
3. Replace the SSH transcript with the new BBS.
4. Validate host and guest flows together.

## Related

- [design-doc/01-sqlite-backed-bubble-tea-bbs-architecture-analysis-and-implementation-guide-for-host-and-guest-runtimes.md](../design-doc/01-sqlite-backed-bubble-tea-bbs-architecture-analysis-and-implementation-guide-for-host-and-guest-runtimes.md)
- [reference/02-implementation-guide.md](./02-implementation-guide.md)
