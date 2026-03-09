---
Title: CGO-backed SQLite persistence and runtime packaging plan for qemu-go-init
Ticket: QEMU-GO-INIT-009
Status: active
Topics:
    - go
    - qemu
    - sqlite
    - pinocchio
    - ssh
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../corporate-headquarters/geppetto/pkg/inference/toolloop/enginebuilder/builder.go
      Note: Upstream TurnPersister hook used to persist final turns
    - Path: ../../../../../../../corporate-headquarters/pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go
      Note: Upstream CGO-backed timeline store to reuse
    - Path: ../../../../../../../corporate-headquarters/pinocchio/pkg/persistence/chatstore/turn_store_sqlite.go
      Note: Upstream CGO-backed turn store to reuse
    - Path: ../../../../../../../corporate-headquarters/pinocchio/pkg/ui/timeline_persist.go
      Note: Upstream timeline projection helper to reuse
    - Path: cmd/init/main.go
      Note: Guest PID 1 entrypoint and runtime bootstrap boundary
    - Path: cmd/mkinitramfs/main.go
      Note: Initramfs packaging path that must stage the dynamic runtime
    - Path: internal/aichat/surface.go
      Note: Current guest chat surface that needs turn and timeline persistence
    - Path: internal/bbsstore/store.go
      Note: Current local SQLite store for comparison against upstream chat persistence
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-09T16:22:28.329175061-04:00
WhatFor: Explain how to migrate qemu-go-init to a CGO-backed guest runtime so the guest can reuse upstream Pinocchio SQLite turn and timeline stores and persist logs.
WhenToUse: Read before implementing or reviewing guest runtime packaging, SQLite persistence wiring, or host and guest log capture.
---


# CGO-backed SQLite persistence and runtime packaging plan for qemu-go-init

## Executive Summary

Today the guest runtime persists only BBS posts, using a local `modernc.org/sqlite` store in [store.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/bbsstore/store.go). The AI chat path does not persist final turns, timeline entities, or logs. Upstream Pinocchio already provides the turn and timeline store implementations we want, but those stores are built on `github.com/mattn/go-sqlite3`, which requires CGO. The central design problem is therefore not only data modeling. It is also runtime packaging: the guest `/init` binary must become a dynamically linked ELF and the initramfs must carry the dynamic loader and required shared libraries so the kernel can execute `/init` at boot.

The design in this ticket keeps the product architecture small and understandable. We will still boot a single guest application entrypoint at `/init`, still use QEMU plus initramfs, and still mount persistent and shared state before higher-level services start. The change is that `/init` becomes a dynamically linked binary, and the initramfs builder explicitly stages the ELF interpreter and shared objects that `ldd` reports for that binary. Once that runtime packaging exists, we can directly reuse upstream Pinocchio’s SQLite turn and timeline stores and add guest log persistence using the same SQLite runtime.

## Problem Statement

The current runtime has four persistence gaps:

- BBS messages are persisted, but AI turns are not.
- Timeline entities emitted during streaming are not persisted.
- Guest application logs are not stored durably.
- QEMU serial logs exist only as host-side text files, not structured persistent records.

There is also a build constraint:

- The guest binary is built with `CGO_ENABLED=0` in [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile), which blocks direct reuse of the upstream SQLite turn and timeline stores in:
  - [turn_store_sqlite.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/persistence/chatstore/turn_store_sqlite.go)
  - [timeline_store_sqlite.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go)

The user explicitly wants us to stop reimplementing these stores locally and instead use the upstream code. That means the guest build has to embrace CGO and package the dynamic runtime correctly.

### Current State Map

The current flow is:

```text
host make run/smoke
  -> build/init (pure Go)
  -> build/initramfs.cpio.gz
  -> qemu boots kernel + initramfs
  -> kernel execs /init
  -> /init mounts filesystems and storage
  -> /init opens bbs.db and starts HTTP + SSH + chat UI
```

The current persistence graph is:

```text
BBS UI / JS REPL
  -> internal/bbsstore.Store
  -> bbs.db

AI chat
  -> in-memory session state only
  -> no turn store
  -> no timeline store
  -> logs only on stdout/stderr
```

The current code points are:

- Guest entrypoint: [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
- Chat surface: [surface.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/aichat/surface.go)
- Current local SQLite store: [store.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/bbsstore/store.go)
- Initramfs builder: [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go)
- Upstream turn persister hook: [builder.go](/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/toolloop/enginebuilder/builder.go)
- Upstream timeline projection helper: [timeline_persist.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/ui/timeline_persist.go)

## Proposed Solution

We will implement the migration in four layers.

### 1. Package a Dynamic Guest Runtime

The guest `/init` binary will be built with `CGO_ENABLED=1` and staged into the initramfs together with:

- the ELF interpreter, for example `/lib64/ld-linux-x86-64.so.2`
- `libc.so.6`
- any additional transitive shared objects reported by `ldd`, such as `libpthread.so.0`, `libm.so.6`, `libdl.so.2`, or `libgcc_s.so.1`

The important boot rule is:

```text
kernel unpacks initramfs
  -> kernel tries to exec /init
  -> loader from initramfs resolves shared libraries from initramfs
  -> only then does Go main() run
```

This means the dynamic runtime must be packaged before boot. It cannot be “mounted later” by the same process.

### 2. Reuse Upstream SQLite Turn and Timeline Stores

We will stop inventing a separate turn or timeline schema in this repo. Instead we will instantiate:

- `chatstore.NewSQLiteTurnStore(...)`
- `chatstore.NewSQLiteTimelineStore(...)`

from the upstream Pinocchio package.

The guest chat surface will:

- create a stable conversation ID and runtime state root
- create a turn persister that implements `enginebuilder.TurnPersister`
- attach the turn persister with `EngineBackend.SetTurnPersister(...)`
- attach timeline persistence by adding `ui.StepTimelinePersistFuncWithVersion(...)` as a router handler on the `"chat"` topic

### 3. Persist Guest Application Logs into SQLite

We will add a local SQLite log store in this repo. This is not available upstream in a form directly shaped for qemu-go-init, so we will implement it locally using the same `github.com/mattn/go-sqlite3` driver that the guest now supports.

The log path will capture:

- zerolog records
- the stdlib logger used by PID 1
- selected structured host status events during boot and service startup

Recommended schema:

```sql
CREATE TABLE logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts_rfc3339 TEXT NOT NULL,
  source TEXT NOT NULL,
  level TEXT NOT NULL,
  component TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL,
  payload_json TEXT NOT NULL DEFAULT '{}'
);
```

We will keep this schema append-only and simple. The critical point is durable debugging, not high-volume observability.

### 4. Capture Host-Side QEMU Serial Logs and Import Them

The “stuff coming out of qemu” is not guest-visible. It is host-side serial console output. We therefore need a host-side capture path.

The host-side plan is:

- ensure `make run` and `make smoke` write QEMU serial output to a stable host file
- add a small host import utility that reads that file and inserts rows into the guest runtime SQLite log database or a companion host-side DB

This split is intentional:

- guest app logs are generated in-guest and can go directly to SQLite
- QEMU serial output is generated on the host and must be captured there first

## Design Decisions

### Decision 1: Reuse upstream Pinocchio stores instead of porting them to `modernc`

This ticket exists because the user wants the upstream stores, not another local reimplementation. Reusing the upstream code minimizes schema drift and keeps future tooling compatibility with Pinocchio.

### Decision 2: Package a dynamic runtime instead of splitting into stage-1 and stage-2 binaries

A static stage-1 init plus a dynamic stage-2 helper would be workable, but it would violate the spirit of the current single-entrypoint runtime and complicate the boot story for new contributors. Packaging the dynamic loader and shared libraries directly into the initramfs keeps boot linear and easier to teach.

### Decision 3: Keep BBS content storage separate from chat runtime storage

The existing `bbs.db` remains useful and small. Turn and timeline persistence will likely live better in a separate runtime DB such as `chat.db` or `runtime.db`, because those schemas are upstream-owned and materially different.

### Decision 4: Store app logs in SQLite, but treat QEMU serial logs as host-originated imports

Trying to make the guest “see” host QEMU stdout directly would be the wrong abstraction. It is better to acknowledge the host boundary and import host logs explicitly.

## Alternatives Considered

### Alternative 1: Port the upstream stores to `modernc.org/sqlite`

This would preserve a pure-Go guest binary, but it would no longer be “use the upstream sqlite stores.” It would also create a maintenance fork. Rejected.

### Alternative 2: Keep turns and timelines in JSON files

This would avoid CGO but lose the benefits of the upstream queryable schema and durable tooling. Rejected.

### Alternative 3: Run a helper process for SQLite after boot

This would let `/init` stay static and launch a dynamic helper later. It would also add IPC and service management complexity. Rejected for this ticket.

## Implementation Plan

### Phase 1: Evidence and Packaging

- Add a probe script under this ticket’s `scripts/` directory that builds a CGO guest binary and records:
  - `file build/init-cgo-probe`
  - `ldd build/init-cgo-probe`
- Extend the initramfs builder to accept file maps for the ELF interpreter and shared libraries.
- Extend the `Makefile` to:
  - build `/init` with `CGO_ENABLED=1`
  - discover runtime library dependencies
  - pass them into `cmd/mkinitramfs`

Pseudocode:

```text
build init-cgo
deps = inspect_elf_dependencies(build/init)
mkinitramfs(
  init=build/init,
  extra_files=deps.loader + deps.shared_libs + ca_bundle + modules
)
```

### Phase 2: Runtime State Layout

Add a stable runtime directory layout under persistent/shared storage:

```text
/var/lib/go-init/shared/bbs
/var/lib/go-init/shared/chat
  turns.db
  timeline.db
  logs.db
  qemu-host.log
```

### Phase 3: Chat Turn and Timeline Persistence

- Add a local turn persister wrapper modeled on:
  - [persistence.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/cmd/switch-profiles-tui/persistence.go)
- Open upstream SQLite stores at startup.
- Wire them into the chat surface:
  - `backend.SetTurnPersister(...)`
  - `router.AddHandler("timeline-persist", "chat", ui.StepTimelinePersistFuncWithVersion(...))`

Sequence diagram:

```text
user prompt
  -> Bobatea chat model
  -> Pinocchio EngineBackend
  -> Geppetto session builder
  -> provider stream events
  -> EventRouter
     -> UI forwarder
     -> timeline persist handler -> timeline.db
  -> final turn completed
     -> TurnPersister -> turns.db
```

### Phase 4: Guest Log Persistence

- Add a SQLite log store and writer adapter.
- Route zerolog output and PID 1 logger output to:
  - stdout/stderr
  - SQLite log store

Pseudocode:

```go
logStore := openLogStore("/var/lib/go-init/shared/chat/logs.db")
writer := io.MultiWriter(os.Stdout, NewSQLiteLogWriter(logStore, "guest"))
zerologOutput = writer
stdlibLogger = log.New(writer, "", flags)
```

### Phase 5: Host QEMU Log Capture and Import

- Write QEMU serial output to a stable host path during `run` and `smoke`.
- Add a small host utility or script to import each line as a SQLite log row with `source = "qemu-host"`.

## Risks

### Risk 1: The required shared library set may differ across hosts

Mitigation:

- discover dependencies from the built binary instead of hardcoding only `libc`
- record the exact `ldd` output in the ticket diary

### Risk 2: CGO complicates builds and CI

Mitigation:

- keep the dynamic packaging logic visible and scripted
- validate the resulting ELF in tests or smoke scripts

### Risk 3: SQLite concurrency across host and guest

Mitigation:

- guest-owned runtime DBs should not be opened concurrently by host-side tooling for writes
- QEMU host log import can run against a separate DB or only when the VM is offline

## Open Questions

- Should host QEMU logs be imported into the same `logs.db` file or a separate host-side DB?
- Should we expose a `/api/debug/persistence` endpoint summarizing row counts and DB paths?
- Do we want to preserve the ability to build a pure-Go guest for comparison, or fully replace it?

## References

- Guest entrypoint: [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
- Chat surface: [surface.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/aichat/surface.go)
- Current local BBS store: [store.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/bbsstore/store.go)
- Initramfs builder: [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go)
- Upstream turn persister hook: [builder.go](/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/toolloop/enginebuilder/builder.go)
- Upstream engine backend: [backend.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/ui/backend.go)
- Upstream timeline persistence: [timeline_persist.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/ui/timeline_persist.go)
- Upstream turn store: [turn_store_sqlite.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/persistence/chatstore/turn_store_sqlite.go)
- Upstream timeline store: [timeline_store_sqlite.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go)
- Closest upstream example: [main.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/cmd/switch-profiles-tui/main.go)
- Closest upstream turn persister helper: [persistence.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/cmd/switch-profiles-tui/persistence.go)

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
