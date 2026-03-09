---
Title: Implementation guide
Ticket: QEMU-GO-INIT-009
Status: active
Topics:
    - go
    - qemu
    - sqlite
    - pinocchio
    - ssh
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: Makefile
      Note: Build and QEMU launch flow that will change for CGO packaging
    - Path: scripts/qemu-smoke.sh
      Note: Current smoke path and host-side QEMU log capture point
    - Path: ttmp/2026/03/09/QEMU-GO-INIT-009--migrate-guest-chat-persistence-and-logging-to-upstream-cgo-backed-sqlite-stores/scripts/cgo-runtime-probe.sh
      Note: Probe script used to inspect runtime dependencies
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-09T16:22:28.458786127-04:00
WhatFor: Give an intern a copy-paste-friendly map of the concrete files, commands, data paths, and APIs involved in migrating qemu-go-init to upstream CGO-backed SQLite stores and SQLite-based log persistence.
WhenToUse: Use while implementing, testing, or reviewing the CGO packaging and persistence migration.
---


# Implementation guide

## Goal

Implement a guest runtime that:

- boots from initramfs even though `/init` is dynamically linked
- persists BBS posts, turns, timeline entities, and logs durably
- reuses upstream Pinocchio SQLite turn and timeline stores
- captures host-side QEMU serial logs into persistent storage

## Context

The current repo already has:

- a QEMU boot flow
- a persistent ext4 data image
- a shared state mount
- an SSH BBS and AI chat surface

What it does not yet have is a CGO-capable guest runtime or upstream chat persistence.

### File Map

- Boot entrypoint:
  - [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
- Initramfs builder:
  - [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go)
- Guest build and QEMU boot commands:
  - [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
- Existing BBS sqlite:
  - [store.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/bbsstore/store.go)
- Guest chat surface:
  - [surface.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/aichat/surface.go)
- Upstream turn persistence hook:
  - [builder.go](/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/toolloop/enginebuilder/builder.go)
- Upstream EngineBackend:
  - [backend.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/ui/backend.go)
- Upstream timeline persistence handler:
  - [timeline_persist.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/ui/timeline_persist.go)

## Quick Reference

### Runtime directories

```text
build/
  init
  initramfs.cpio.gz
  data.img
  shared-state/
    bbs/
      bbs.db
    pinocchio/
      config.yaml
      profiles.yaml
    chat/
      qemu-host-logs.db

/var/lib/go-init/
  state/
    chat/
      turns.db
      timeline.db
      logs.db
```

### Required wiring for turns

```go
turnStore, _ := chatstore.NewSQLiteTurnStore(dsn)
persister := newTurnStorePersister(turnStore, convID)
backend.SetTurnPersister(persister)
```

### Required wiring for timeline

```go
timelineStore, _ := chatstore.NewSQLiteTimelineStore(dsn)
router.AddHandler(
  "timeline-persist",
  "chat",
  ui.StepTimelinePersistFuncWithVersion(timelineStore, convID, &timelineVersion),
)
```

### Required wiring for logs

```go
logStore, _ := logstore.Open("/var/lib/go-init/state/chat/logs.db")
writer := io.MultiWriter(os.Stdout, logStore.Writer("guest"))
zerolog.Logger = zerolog.New(writer)
pid1Logger := log.New(writer, "", log.LstdFlags|log.Lmicroseconds|log.LUTC)
```

### Required packaging rule

If `ldd build/init` prints a dependency, that dependency must be in the initramfs at boot time.

Example:

```text
/init
  -> /lib64/ld-linux-x86-64.so.2
  -> /lib/x86_64-linux-gnu/libc.so.6
  -> /lib/x86_64-linux-gnu/libpthread.so.0
  -> /lib/x86_64-linux-gnu/libm.so.6
  -> /lib/x86_64-linux-gnu/libdl.so.2
  -> /lib/x86_64-linux-gnu/libgcc_s.so.1
```

### Suggested task order

1. Prove the CGO binary builds and inspect `ldd`.
2. Package the interpreter and shared objects into initramfs.
3. Boot the dynamic guest.
4. Add upstream turn store.
5. Add upstream timeline store.
6. Add guest log store.
7. Add host-side QEMU log import.

## Usage Examples

### Example: inspect dynamic dependencies

```bash
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -o build/init-cgo-probe ./cmd/init
file build/init-cgo-probe
ldd build/init-cgo-probe
./scripts/collect-elf-runtime.sh build/init-cgo-probe
```

### Example: boot after packaging shared libraries

```bash
make run INIT_CGO_ENABLED=1 KERNEL_IMAGE=qemu-vmlinuz QEMU_HOST_PORT=18088 QEMU_SSH_HOST_PORT=10030
```

### Example: verify persistence

```bash
curl -fsS http://127.0.0.1:18097/api/debug/aichat/runtime
curl -fsS http://127.0.0.1:18097/api/debug/logs/runtime
sqlite3 build/shared-state-cgo-009/chat/qemu-host-logs.db 'select count(*) from logs;'
```

### Example: prove the dynamic guest still boots

```bash
make QEMU_DATA_IMAGE=build/data-cgo.img data-image
make smoke \
  INIT_CGO_ENABLED=1 \
  KERNEL_IMAGE=qemu-vmlinuz \
  QEMU_HOST_PORT=18090 \
  QEMU_SSH_HOST_PORT=10032 \
  QEMU_DATA_IMAGE=build/data-cgo.img \
  QEMU_SHARED_STATE_HOST_PATH=build/shared-state-cgo
```

### Example: drive one real chat turn and confirm guest persistence counts

```bash
ssh -tt \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o PreferredAuthentications=none \
  -o PubkeyAuthentication=no \
  -o PasswordAuthentication=no \
  -p 10039 \
  127.0.0.1
```

Inside the BBS:

- press `c` to enter AI chat
- type a prompt
- press `Tab` to submit
- press `Ctrl+B` to return to the BBS

Then confirm persistence:

```bash
curl -fsS http://127.0.0.1:18097/api/debug/aichat/runtime
```

Expected fields after one successful chat turn:

- `turnsCount: 1`
- `timelineConversationCount: 1`
- `timelineVersionCount: 1`
- `timelineEntityCount: 2`

## Related

- Design: [01-cgo-backed-sqlite-persistence-and-runtime-packaging-plan-for-qemu-go-init.md](../design-doc/01-cgo-backed-sqlite-persistence-and-runtime-packaging-plan-for-qemu-go-init.md)
- Diary: [01-diary.md](./01-diary.md)
