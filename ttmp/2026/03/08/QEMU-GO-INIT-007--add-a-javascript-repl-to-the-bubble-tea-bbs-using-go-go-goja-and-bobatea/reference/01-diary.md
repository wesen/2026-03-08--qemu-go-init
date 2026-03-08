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
- the adapter path is not guest-safe for this repo because it pulls in tree-sitter parser code that breaks the static `CGO_ENABLED=0` build
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
10. Added repo-local module wiring and the first implementation of `internal/jsrepl`, initially using the higher-level `go-go-goja` Bobatea adapter.
11. Refactored `internal/bbsapp` into a pointer-owned model so it could hold long-lived REPL state and attach the concrete `*tea.Program`.
12. Updated `cmd/bbs` and `internal/sshbbs` so both the host-native and Wish SSH paths attach the Bobatea UI forwarder correctly.
13. First `make smoke` failed during the static guest build with:

```text
github.com/tree-sitter/tree-sitter-javascript/bindings/go: build constraints exclude all Go files
```

14. Root cause: the imported `go-go-goja` JavaScript REPL adapter brings in parser/autocomplete code that depends on tree-sitter bindings, and that path is not viable for the static guest `/init` binary.
15. Pivoted to a repo-owned Bobatea evaluator that uses `go-go-goja/engine` directly and implements the small `bobatea/pkg/repl.Evaluator` interface in-repo.
16. Preserved the `bbs` JS API by exporting hidden Go callbacks and loading a tiny JS shim that converts JSON payloads into normal JavaScript arrays and objects.
17. Re-ran `go test ./... -count=1`; it passed.
18. Re-ran `make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18084 QEMU_SSH_HOST_PORT=10026`; it passed.
19. Ran `docmgr doctor --ticket QEMU-GO-INIT-007 --stale-after 30`; it passed.
20. Uploaded the ticket bundle to reMarkable and verified `QEMU-GO-INIT-007` under `/ai/2026/03/08/`.

## Related

- `../02-implementation-guide.md`
- `../../design-doc/01-javascript-repl-architecture-and-implementation-guide-for-the-bubble-tea-bbs.md`
