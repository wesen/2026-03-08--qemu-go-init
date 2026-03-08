---
Title: Diary
Ticket: QEMU-GO-INIT-007
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - ssh
    - tui
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T19:32:07.01729177-04:00
WhatFor: "Chronological implementation diary for the BBS JavaScript REPL ticket."
WhenToUse: "Read when you need the exact sequence of experiments, findings, errors, and code/documentation changes made during ticket 007."
---

# Diary

## Goal

Record the detailed day-of implementation trail for adding an embedded JavaScript REPL to the BBS. This diary is intentionally operational: it captures what was tried, what APIs were inspected, what assumptions were confirmed, and what changed in the repository and ticket workspace.

## Context

The starting point for this ticket was:

- `QEMU-GO-INIT-006` delivered a shared-state Bubble Tea BBS backed by SQLite.
- The BBS already runs in two environments:
  - host-native via `cmd/bbs`
  - guest-native via Wish over SSH
- Persistence is already available through the shared-state mount.
- The next requested capability is an embedded JavaScript REPL using local `go-go-goja` and `bobatea`.

## Quick Reference

### Key findings so far

- `go-go-goja` already provides a Bobatea adapter:
  - `pkg/repl/adapters/bobatea/javascript.go`
- Bobatea REPL is not just a local child widget:
  - it needs an event bus
  - it needs `repl.RegisterReplToTimelineTransformer(bus)`
  - it needs `timeline.RegisterUIForwarder(bus, program)`
- Wish’s simple middleware API is insufficient for this exact integration:
  - use `wishbubbletea.MiddlewareWithProgramHandler(...)`
- Charm rendering does not require shipping a separate terminfo/termios database for this project’s normal TUI path.

### Intended architecture

```text
BBS model
  -> browse mode
  -> compose mode
  -> REPL mode
       -> jsrepl wrapper
            -> go-go-goja evaluator
            -> bobatea repl model
            -> bobatea bus + ui forwarder
```

## Usage Examples

### Useful local inspection commands used during research

```bash
sed -n '1,240p' internal/bbsapp/model.go
sed -n '1,220p' internal/sshbbs/middleware.go
sed -n '1,240p' /home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/repl/adapters/bobatea/javascript.go
sed -n '1,260p' /home/manuel/code/wesen/corporate-headquarters/bobatea/pkg/repl/model.go
sed -n '1,220p' $(go env GOPATH)/pkg/mod/github.com/charmbracelet/wish@v1.4.7/bubbletea/tea.go
```

### Chronological log

1. Created ticket `QEMU-GO-INIT-007` with `docmgr ticket create-ticket`.
2. Corrected an initial CLI flag mistake: the command expects `--ticket`, not `--id`.
3. Inspected the current BBS and Wish middleware to locate the exact program ownership boundary.
4. Inspected local `go-go-goja` and `bobatea` packages to confirm that a Bobatea adapter already exists for the JavaScript evaluator.
5. Confirmed that the SSH path must move to `MiddlewareWithProgramHandler` because the Bobatea UI forwarder must bind to the concrete `*tea.Program`.
6. Wrote the initial ticket architecture, tasks, and implementation notes before touching repo code.
7. Added `scripts/js-repl-probe` with local `replace` directives for the requested corporate-headquarters packages.
8. First probe attempt failed under the default toolchain because local `bobatea` requires Go `1.25.7` while the repo is on `1.25.5`.
9. Retried the probe with `GOTOOLCHAIN=auto`, which succeeded and produced:
   - evaluator name: `JavaScript`
   - prompt: `js>`
   - a streamed result event for `nums.reduce(...)` returning `6`

## Related

- `../02-implementation-guide.md`
- `../../design-doc/01-javascript-repl-architecture-and-implementation-guide-for-the-bubble-tea-bbs.md`
