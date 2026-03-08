---
Title: Add a JavaScript REPL to the Bubble Tea BBS using go-go-goja and bobatea
Ticket: QEMU-GO-INIT-007
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - ssh
    - tui
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T19:31:08.929553279-04:00
WhatFor: "Track design and implementation of an embedded JavaScript REPL mode inside the shared-state Bubble Tea BBS for both host-native and Wish-over-SSH execution paths."
WhenToUse: "Use this ticket when changing the REPL mode, Bubble Tea/Wish integration, local go-go-goja and bobatea dependency strategy, or operator test flow for the SSH-accessible BBS."
---

# Add a JavaScript REPL to the Bubble Tea BBS using go-go-goja and bobatea

## Overview

This ticket adds a JavaScript REPL mode to the existing SQLite-backed Bubble Tea BBS. The REPL must run inside the same TUI that already powers the host-native `cmd/bbs` program and the guest-facing Wish SSH application.

The implementation target is intentionally local-first:

- Use `/home/manuel/code/wesen/corporate-headquarters/go-go-goja` for JavaScript evaluation.
- Use `/home/manuel/code/wesen/corporate-headquarters/bobatea` for the REPL/timeline TUI shell.
- Keep the BBS as the top-level application and embed the REPL as another mode instead of spawning a separate process.
- Ensure both the host process and the Wish SSH process attach the Bobatea UI forwarder to the actual `*tea.Program`.

Current state:

- Ticket workspace created.
- Existing repo already has a shared-state Bubble Tea BBS and Wish SSH app.
- Integration design verified by reading local `go-go-goja`, `bobatea`, and Wish `bubbletea` APIs.
- Next deliverables are a probe in `scripts/`, source implementation, and validation docs.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field
- **Design**: [design-doc/01-javascript-repl-architecture-and-implementation-guide-for-the-bubble-tea-bbs.md](./design-doc/01-javascript-repl-architecture-and-implementation-guide-for-the-bubble-tea-bbs.md)
- **Implementation Guide**: [reference/02-implementation-guide.md](./reference/02-implementation-guide.md)
- **Diary**: [reference/01-diary.md](./reference/01-diary.md)

## Status

Current status: **active**

## Topics

- go
- qemu
- linux
- initramfs
- ssh
- tui

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
