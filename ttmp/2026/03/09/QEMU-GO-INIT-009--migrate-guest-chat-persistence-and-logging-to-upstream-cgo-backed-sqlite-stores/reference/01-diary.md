---
Title: Diary
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
    - Path: ../../../../../../../corporate-headquarters/pinocchio/cmd/switch-profiles-tui/main.go
      Note: Closest upstream persistence integration example consulted during investigation
    - Path: ../../../../../../../corporate-headquarters/pinocchio/cmd/switch-profiles-tui/persistence.go
      Note: Closest upstream turn persister helper consulted during investigation
    - Path: ttmp/2026/03/09/QEMU-GO-INIT-009--migrate-guest-chat-persistence-and-logging-to-upstream-cgo-backed-sqlite-stores/design-doc/01-cgo-backed-sqlite-persistence-and-runtime-packaging-plan-for-qemu-go-init.md
      Note: Primary design narrative referenced by diary steps
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-09T16:22:28.448826317-04:00
WhatFor: Capture the implementation journey for the CGO guest runtime and upstream SQLite persistence migration, including build experiments, boot failures, packaging details, and review notes.
WhenToUse: Read when continuing the migration, reviewing the implementation, or reconstructing why a particular packaging or persistence choice was made.
---


# Diary

## Goal

Record the migration from the current pure-Go guest runtime to a dynamically packaged CGO guest runtime that can reuse the upstream Pinocchio SQLite turn and timeline stores and persist application and QEMU logs.

## Step 1: Define the migration target and gather the upstream evidence

The first step was to turn the user’s request into a concrete technical plan. The request was not only “persist timelines and turns.” It explicitly required using the upstream SQLite stores and adding log persistence. That immediately forced the CGO packaging question, because the upstream Pinocchio SQLite stores are based on `github.com/mattn/go-sqlite3` rather than the pure-Go `modernc.org/sqlite` driver currently used in qemu-go-init.

I gathered the exact upstream hooks and compared them to the current guest runtime. The core finding is that we do not need to invent a new persistence model. The turn and timeline persistence mechanisms already exist upstream. The real work is packaging a dynamic `/init`, opening the right databases from guest storage, and routing guest and host logs into durable storage without confusing the host or guest boundaries.

### Prompt Context

**User prompt (verbatim):** "ok, create a ticket to do that, create a detailed implementation plan so we use the upstream sqlite stores, store the logs of the app in sqlite too, and use the CGO version of sqlite. 

Create a detailed analysis / design / implementation guide that is very detailed for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file
  references.
  It should be very clear and detailed. Store in the ticket and the nupload to remarkable.

Then add tasks, and work on task one by one, commit at appropriate itmes, and keep a detailed diary"

**Assistant interpretation:** Create a new docmgr ticket, write the long-form design and implementation docs, then migrate qemu-go-init to a CGO-backed guest runtime that reuses upstream Pinocchio SQLite stores and persists application logs, while documenting each implementation slice and committing incrementally.

**Inferred user intent:** Replace the ad hoc local persistence model with the real upstream persistence stack, keep the guest bootable inside QEMU, and make the whole change explainable and reviewable for future contributors.

### What I did

- Inspected the current guest entrypoint in [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go).
- Confirmed that the current persistent DB is only the BBS message store in [store.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/bbsstore/store.go).
- Inspected the upstream turn persister hook in [builder.go](/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/toolloop/enginebuilder/builder.go).
- Inspected the upstream timeline persistence handler in [timeline_persist.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/ui/timeline_persist.go).
- Inspected the upstream SQLite stores in:
  - [turn_store_sqlite.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/persistence/chatstore/turn_store_sqlite.go)
  - [timeline_store_sqlite.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go)
- Inspected the closest upstream integration example in:
  - [main.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/cmd/switch-profiles-tui/main.go)
  - [persistence.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/cmd/switch-profiles-tui/persistence.go)
- Created ticket `QEMU-GO-INIT-009` and drafted the design doc, implementation guide, and this diary.

### Why

- We needed file-backed evidence before committing to the migration path.
- We needed to verify that upstream already solves most of the persistence layer so we could avoid inventing a second schema.
- We needed to document the boot-time consequence of moving to CGO: `/init` becomes a dynamically linked ELF and therefore requires its loader and shared libraries in the initramfs before boot.

### What worked

- The upstream persistence model matched the user’s request closely.
- The current initramfs builder already supports arbitrary extra file mappings, which is enough in principle to package shared libraries and the ELF interpreter.
- The current shared state and persistent storage layout already gives us a place to put `turns.db`, `timeline.db`, and `logs.db`.

### What didn't work

- A live `CGO_ENABLED=1` build probe for `./cmd/init` did not finish immediately; it remained running longer than the pure-Go build. That is not yet a failure, but it is a signal that the first implementation slice must explicitly validate the build and record the resulting dynamic dependency set.

Command used:

```bash
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -o build/init-cgo-probe ./cmd/init
```

Observed process:

```text
go build -trimpath -o build/init-cgo-probe ./cmd/init
```

### What I learned

- The upstream Pinocchio stores are directly reusable, but only if the guest runtime accepts CGO.
- The current repo already has enough initramfs machinery to package arbitrary runtime files; we do not need a new archive format or a new build tool.
- Host-side QEMU serial logs are a different class of logs from guest application logs and should be modeled as an import path, not as in-guest log generation.

### What was tricky to build

The subtle part was separating “use upstream stores” from “keep a single binary OS.” A dynamically linked `/init` is still one binary from the user’s perspective, but it is no longer self-sufficient in the initramfs. The kernel must load the ELF interpreter before `main()` runs. That means the packaging work is not optional glue. It is part of boot correctness.

### What warrants a second pair of eyes

- Whether the final guest runtime should be fully replaced by CGO, or whether we keep a pure-Go build target for comparison.
- Whether host QEMU log import should target the same `logs.db` file as guest app logs or a separate DB to avoid concurrent write surprises.
- Whether the dynamic library discovery step should be fully automatic or controlled by a checked-in allowlist plus verification.

### What should be done in the future

- Run and record the CGO build probe to completion.
- Implement the dynamic runtime packaging path.
- Wire the upstream turn and timeline stores.
- Add the guest log store and the host QEMU log importer.

### Code review instructions

- Start with the design doc:
  - [01-cgo-backed-sqlite-persistence-and-runtime-packaging-plan-for-qemu-go-init.md](../design-doc/01-cgo-backed-sqlite-persistence-and-runtime-packaging-plan-for-qemu-go-init.md)
- Then review the current runtime and store boundaries:
  - [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
  - [store.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/bbsstore/store.go)
  - [surface.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/aichat/surface.go)
- Validate the upstream hook points:
  - [builder.go](/home/manuel/code/wesen/corporate-headquarters/geppetto/pkg/inference/toolloop/enginebuilder/builder.go)
  - [timeline_persist.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/ui/timeline_persist.go)

### Technical details

Conceptual boot flow for the target design:

```text
host build
  -> dynamic /init
  -> initramfs includes /init + ld-linux + libc + other .so files
  -> qemu boots kernel + initramfs
  -> kernel executes /init through loader
  -> /init mounts storage/shared state
  -> /init opens turns.db, timeline.db, logs.db
  -> chat + log persistence become durable
```

Candidate runtime DB layout:

```text
/var/lib/go-init/shared/chat/
  turns.db
  timeline.db
  logs.db
  qemu-host.log
```
