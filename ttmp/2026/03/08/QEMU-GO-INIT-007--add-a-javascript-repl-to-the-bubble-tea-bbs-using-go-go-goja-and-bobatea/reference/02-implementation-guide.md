---
Title: Implementation guide
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
LastUpdated: 2026-03-08T19:32:07.350681625-04:00
WhatFor: "Provide an intern-friendly implementation guide for the JavaScript REPL feature."
WhenToUse: "Use when implementing, testing, or extending the embedded BBS JavaScript REPL on either the host-native or Wish-over-SSH path."
---

# Implementation guide

## Goal

Explain exactly how to add and validate the embedded JavaScript REPL in a way a new intern can follow without already knowing Bubble Tea, Wish, Bobatea, or go-go-goja.

## Context

### What this system is

This repository boots a Linux guest in QEMU using a Go binary as `/init`. That binary mounts a minimal runtime environment, starts an HTTP server, starts a Wish SSH server, and exposes a Bubble Tea BBS over SSH.

The BBS state is stored in SQLite inside the shared-state mount. The new REPL feature extends that BBS with a programmable console.

### What each important subsystem does

- `cmd/init`
  - guest PID 1 runtime
  - boots the guest services
- `internal/sshapp`
  - general Wish server construction
- `internal/sshbbs`
  - adapts the BBS into the Wish SSH app
- `cmd/bbs`
  - host-native entry point for the same BBS app
- `internal/bbsapp`
  - Bubble Tea BBS UI logic
- `internal/bbsstore`
  - SQLite store for posts
- local `go-go-goja`
  - JavaScript evaluation engine
- local `bobatea`
  - transcript-style REPL UI shell

## Quick Reference

### File map

- Repo BBS model: `/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/bbsapp/model.go`
- Repo host BBS command: `/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/bbs/main.go`
- Repo Wish BBS middleware: `/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshbbs/middleware.go`
- Local JS evaluator adapter: `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/repl/adapters/bobatea/javascript.go`
- Local Bobatea REPL model: `/home/manuel/code/wesen/corporate-headquarters/bobatea/pkg/repl/model.go`

### Minimum integration checklist

- add module dependencies and local `replace` directives
- create a wrapper package for evaluator + bus + model
- add REPL mode to the BBS
- attach the `*tea.Program` in both host and SSH launch paths
- validate both launch paths

### Important APIs

#### go-go-goja

```go
evaluator, err := jsadapter.NewJavaScriptEvaluatorWithDefaults()
defer evaluator.Close()
```

#### bobatea

```go
bus, err := eventbus.NewInMemoryBus()
repl.RegisterReplToTimelineTransformer(bus)
model := repl.NewModel(evaluator, cfg, bus.Publisher)
timeline.RegisterUIForwarder(bus, program)
go bus.Run(ctx)
```

#### wish

```go
wishbubbletea.MiddlewareWithProgramHandler(func(sess ssh.Session) *tea.Program {
    ...
})
```

### Control-flow diagram

```text
cmd/bbs or Wish SSH session
        |
        v
  construct BBS model
        |
        v
  construct tea.Program
        |
        +--> attach jsrepl bus/UI forwarder to that program
        |
        v
  run Bubble Tea program
```

## Usage Examples

### Example: probe the evaluator outside the main repo

Create a small experiment under this ticket’s `scripts/` directory, use local `replace` directives, and verify:

- the evaluator instantiates
- the Bobatea REPL model instantiates
- the bus runs
- one evaluation can emit transcript events

### Example: host-native validation target

```bash
go run ./cmd/bbs -state-root build/shared-state/bbs
```

Expected user flow:

- start in browse mode
- press the configured key to enter REPL mode
- run a trivial JS expression such as `2 + 2`
- leave REPL mode and return to the message board

### Example: SSH validation target

```bash
ssh -tt \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o PreferredAuthentications=none \
  -o PubkeyAuthentication=no \
  -o PasswordAuthentication=no \
  -p 10026 \
  127.0.0.1
```

Expected user flow:

- Wish session starts the BBS
- REPL mode can be entered without crashing
- transcript output renders in the SSH PTY
- leaving the REPL returns to the BBS instead of exiting the whole app

## Related

- `./01-diary.md`
- `../../design-doc/01-javascript-repl-architecture-and-implementation-guide-for-the-bubble-tea-bbs.md`
