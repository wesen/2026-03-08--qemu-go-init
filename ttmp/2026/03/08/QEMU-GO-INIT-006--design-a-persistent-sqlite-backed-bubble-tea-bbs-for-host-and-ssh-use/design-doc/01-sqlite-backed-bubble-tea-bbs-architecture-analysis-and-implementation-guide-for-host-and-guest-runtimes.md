---
Title: SQLite-backed Bubble Tea BBS architecture, analysis, and implementation guide for host and guest runtimes
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
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Detailed architecture and implementation design for a shared-state Bubble Tea BBS that runs on the host and inside the QEMU guest over SSH.
LastUpdated: 2026-03-08T19:08:00-04:00
WhatFor: Detailed architecture and implementation design for a shared-state Bubble Tea BBS that runs on the host and inside the QEMU guest over SSH.
WhenToUse: Read this before implementing the BBS, the shared host-guest storage path, or terminal rendering behavior.
---

# SQLite-backed Bubble Tea BBS architecture, analysis, and implementation guide for host and guest runtimes

## Executive Summary

We want the QEMU guest to stop presenting a static SSH status transcript and instead present a small interactive BBS. The same BBS code should also run as a host-native program so developers can browse and write messages without booting QEMU. The BBS will use SQLite for persistence, Bubble Tea for the TUI state machine, and Wish for SSH transport inside the guest.

The key storage design decision is that BBS data must live in a host directory that the guest mounts, not only in the guest's raw disk image. That gives us one shared content location, one schema, and one application package. In this environment, the realistic implementation is a `9p` shared directory mounted by the guest at boot. `virtiofs` is still the preferred future upgrade, but it is blocked here by the absence of `virtiofsd`.

## Problem Statement

The current system already has:

- a static Go PID 1 guest runtime
- DHCP and early-boot entropy support
- persistent guest-local block storage
- a Wish-based SSH service

What it does not have is an actual application with durable content and a proper terminal UI. The existing SSH service only renders a diagnostic transcript and exits. That is useful for smoke testing, but it is not the intended product direction.

The user wants a BBS that can:

- browse messages
- write messages
- persist them
- run locally on the host
- run over SSH inside the guest

If we stored the BBS database only inside the guest's raw `ext4` image, the guest could persist it across reboot, but the host-native BBS binary would not be able to open the same file directly without loop-mount tooling or another translation layer. That would undermine the requirement that the host binary and the guest app use the same content store naturally.

## Proposed Solution

Implement the system as five cooperating layers.

### 1. Shared-State Transport

Add a second persistence path besides the existing block-backed guest storage:

- host directory on the real machine, for example `build/shared-state`
- QEMU passes that directory into the guest with `-virtfs`
- guest loads `9p`, `9pnet`, and `9pnet_virtio` modules
- guest mounts the exported directory at a stable path such as `/var/lib/go-init/shared`

This path is specifically for content that should be visible both to:

- the host-native `cmd/bbs` program
- the guest SSH BBS

### 2. SQLite Store

Create a store package that:

- opens `bbs.db` inside the shared-state directory
- ensures the schema exists
- exposes small focused methods like `ListMessages`, `CreateMessage`, and `GetMessage`
- enables sane PRAGMAs for this use case

Recommended phase-1 schema:

- one table for `messages`
- append-only semantics
- no login system yet
- no threaded replies yet

### 3. Reusable Bubble Tea Application

Build a Bubble Tea model package that is storage-agnostic except for calling the store interface. The same app package should support:

- host terminal execution
- Wish SSH execution

The model should start simple:

- message list pane
- detail pane
- composer flow for new posts
- status/footer line with keybindings

### 4. Host Entry Point

Add `cmd/bbs/main.go` that:

- resolves a state root on the host, defaulting to something like `build/shared-state`
- opens the SQLite store there
- starts the Bubble Tea program on the host TTY

### 5. SSH Adapter

Replace the current transcript middleware with Wish Bubble Tea integration using `github.com/charmbracelet/wish/bubbletea`. The SSH service should:

- open the same shared-state path inside the guest mount
- start a Bubble Tea program for the SSH session
- optionally keep the existing HTTP diagnostics page as an out-of-band health surface

### Architecture Diagram

```text
Host OS
  |
  | runs qemu-system-x86_64
  | runs cmd/bbs locally
  v
+-----------------------------+
| host directory: shared-state|
|   bbs.db                    |
|   uploads/ (later)          |
+-----------------------------+
          |                ^
          | qemu -virtfs   | native host file access
          v                |
+----------------------------------------------+
| QEMU guest                                    |
|  kernel + initramfs + Go PID 1                |
|                                               |
|  mount 9p share -> /var/lib/go-init/shared    |
|  Wish SSH server                              |
|      -> Bubble Tea BBS                        |
|      -> SQLite store on shared path           |
+----------------------------------------------+
```

### Program Shape Diagram

```text
cmd/bbs --------------------------+
                                  |
                                  v
                          internal/bbsapp
                                  |
                                  v
                          internal/bbsstore
                                  |
                                  v
                              SQLite

cmd/init -> internal/sshapp -> internal/sshbbs -> internal/bbsapp -> internal/bbsstore -> SQLite
```

## Design Decisions

### Decision: use a shared host directory for BBS content

Rationale:

- satisfies the "same app on host and guest" requirement directly
- avoids host loop-mounting into a raw guest disk image
- keeps the content store inspectable during development

### Decision: use `9p` now, keep `virtiofs` as the preferred future upgrade

Rationale:

- `virtiofsd` is not installed on this host
- the current guest kernel already has `9p` support as modules
- we can bundle those modules into the initramfs with the existing module-packaging approach

Tradeoff:

- `9p` is not the strongest long-term filesystem choice for SQLite-heavy workloads
- we should document that simultaneous host and guest write access is not the phase-1 contract

### Decision: use `modernc.org/sqlite`

Rationale:

- works in pure Go
- compatible with the static `CGO_ENABLED=0` build used by the current guest pipeline
- avoids dragging a C toolchain into the initramfs story

### Decision: use Bubble Tea and Wish Bubble Tea integration rather than hand-rolled ANSI I/O

Rationale:

- cleaner event model
- easier reuse between host and SSH session transports
- better fit for the "actual TUI app" goal

### Decision: keep phase 1 schema intentionally small

Rationale:

- a single-board message list is enough to validate storage, TUI layout, SSH transport, and host reuse
- users, replies, moderation, pagination, and uploads can come later

### Decision: keep the existing HTTP page

Rationale:

- it remains useful for smoke tests and introspection even after SSH becomes a real TUI app
- it can expose the shared-state mount status and BBS DB path

## Alternatives Considered

### Store the SQLite database only in `build/data.img`

Rejected for this ticket because:

- the guest can use it naturally
- the host-native BBS cannot open the same file directly without extra mounting steps

This is fine for guest-only persistence, not for a host+guest shared content workflow.

### Use `virtiofs` immediately

Preferred in theory, rejected for now because:

- the host environment currently does not provide `virtiofsd`

If `virtiofsd` becomes available later, the app/store layers should not need major changes.

### Use `9p` but keep the host binary read-only

Possible, but not chosen because:

- the user wants to browse and write messages on the host too

We will still document that concurrent host+guest writes are a risk and that phase 1 expects one active writer path at a time.

### Build a bespoke file or replication protocol instead of using a shared filesystem

Rejected as too much complexity for the current stage. A small BBS does not need a distributed state system yet.

## Implementation Plan

### Step 1. Ticket and documentation update

- update the ticket docs to explain host/guest terminology
- document the shift from raw-image-only persistence to shared host-directory persistence
- record the `9p` versus `virtiofs` decision explicitly

### Step 2. Shared-state mount support

- add config for a shared-state backend in the guest runtime
- bundle `9p` modules into the initramfs
- load the modules during boot
- mount the exported directory at `/var/lib/go-init/shared`
- expose mount status in the web UI

Pseudocode:

```text
sharedCfg := sharedstate.LoadConfigFromEnv()
sharedResult := sharedstate.Prepare(logger, sharedCfg)
if sharedResult.Required && !sharedResult.Mounted:
    halt
```

### Step 3. SQLite store package

- create `internal/bbsstore`
- open `<stateRoot>/bbs.db`
- run schema migrations on startup
- add CRUD methods for simple messages

Pseudocode:

```text
db = sql.Open("sqlite", dbPath)
exec PRAGMA foreign_keys = ON
exec PRAGMA busy_timeout = 5000
exec schema statements
return Store{db}
```

### Step 4. Bubble Tea application package

- create `internal/bbsapp`
- define the model, messages, key handling, and views
- implement list/detail/composer states
- keep the rendering logic transport-independent

Pseudocode:

```text
type Model struct {
    store Store
    posts []Message
    cursor int
    mode string
    composer textarea.Model
}
```

### Step 5. Host CLI

- add `cmd/bbs`
- default state root to a host path such as `build/shared-state`
- run the Bubble Tea program with the host terminal

### Step 6. Wish SSH integration

- add `internal/sshbbs`
- use Wish Bubble Tea middleware
- open the store against the guest's shared-state mount
- serve the BBS as the SSH session

### Step 7. Validation and smoke coverage

- host: create/read messages through `cmd/bbs`
- guest: connect over SSH and inspect the same content
- verify a post created in one path appears in the other after restart

### File Map

- [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
- [internal/sshapp/server.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/server.go)
- [internal/storage/storage.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/storage/storage.go)
- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)

Expected new files:

- [cmd/bbs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/bbs/main.go)
- [internal/bbsstore/store.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/bbsstore/store.go)
- [internal/bbsapp/model.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/bbsapp/model.go)
- [internal/sshbbs/middleware.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshbbs/middleware.go)
- [internal/sharedstate/sharedstate.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sharedstate/sharedstate.go)

## Open Questions

### Do Bubble Tea and Lip Gloss need a separate terminfo or termios database in the guest?

Short answer:

- no extra ncurses package or terminfo package needs to be installed in the guest for this project
- the stack is self-contained enough in Go for our initramfs runtime

Longer answer:

- Bubble Tea talks to the terminal using Go and OS terminal primitives
- Lip Gloss and related libraries use Go packages such as `termenv`, `colorprofile`, and `xo/terminfo` to reason about terminal capabilities
- that means the stack is not conceptually "database-free", but the capability logic ships in Go dependencies rather than requiring a full classic terminal userland

### Is SQLite on a shared `9p` mount safe?

It is acceptable for a controlled development prototype if we avoid simultaneous host and guest writes. It is not the ideal long-term database filesystem.

Phase-1 expectation:

- one active writer path at a time
- careful documentation
- smoke tests for sequential access

### Should host keys move to the shared directory too?

Not necessarily. Guest host keys can remain on the guest-local persistent volume. The BBS content store is the thing that needs natural host visibility.

## References

- Bubble Tea: https://github.com/charmbracelet/bubbletea
- Wish Bubble Tea middleware: https://pkg.go.dev/github.com/charmbracelet/wish/bubbletea
- Lip Gloss: https://github.com/charmbracelet/lipgloss
- termenv: https://pkg.go.dev/github.com/muesli/termenv
- xo/terminfo: https://pkg.go.dev/github.com/xo/terminfo
- modernc SQLite: https://pkg.go.dev/modernc.org/sqlite
- SQLite WAL overview: https://sqlite.org/wal.html
