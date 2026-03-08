---
Title: Design a persistent SQLite-backed Bubble Tea BBS for host and SSH use
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
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Shared workspace for designing and implementing a Bubble Tea BBS that runs both natively on the host and as the Wish SSH application inside the QEMU guest.
LastUpdated: 2026-03-08T19:08:00-04:00
WhatFor: Plan and implementation workspace for a shared-state Bubble Tea BBS using SQLite, Bubble Tea, Wish, and QEMU pass-through storage.
WhenToUse: Use this ticket when working on the BBS product shape, the shared host-guest storage path, SSH/TUI integration, or terminal capability questions.
---

# Design a persistent SQLite-backed Bubble Tea BBS for host and SSH use

## Overview

This ticket moves the project from a status-style SSH transcript app to a real interactive application. The target system is a small single-board BBS backed by SQLite, rendered with Bubble Tea and Lip Gloss, and reachable in two modes:

- as the SSH application shown when connecting to the QEMU guest through Wish
- as a host-native CLI binary that uses the same BBS codepath and the same content store

The main architectural update for this ticket is that BBS content should not live only inside the guest's raw `ext4` disk image. Instead, the BBS database and associated content should live in a host directory that QEMU passes into the guest as a shared mount. In the current environment, `virtiofs` would be the preferred long-term mechanism, but `virtiofsd` is not installed. The practical implementation path is therefore:

- phase 1: shared host directory mounted in the guest over `9p`
- phase 2 later: optional migration to `virtiofs` if the host environment provides `virtiofsd`

This lets the host binary open the BBS database directly while the guest SSH application sees the same files at a guest mount point.

## Key Links

- Design doc: [design-doc/01-sqlite-backed-bubble-tea-bbs-architecture-analysis-and-implementation-guide-for-host-and-guest-runtimes.md](./design-doc/01-sqlite-backed-bubble-tea-bbs-architecture-analysis-and-implementation-guide-for-host-and-guest-runtimes.md)
- Implementation guide: [reference/02-implementation-guide.md](./reference/02-implementation-guide.md)
- Diary: [reference/01-diary.md](./reference/01-diary.md)

## Status

Current status: **active**

Current implementation direction:

- shared-state mount: `9p`
- database engine: `modernc.org/sqlite`
- host runtime: native Go CLI
- guest runtime: Wish + Bubble Tea
- current phase: planning and first implementation slice

## Topics

- go
- qemu
- linux
- initramfs
- ssh
- tui
- sqlite

## Tasks

See [tasks.md](./tasks.md) for the current task list.

Current execution order:

1. Update the plan from raw-image-only persistence to shared host-directory persistence.
2. Add shared-state mount plumbing and module loading for `9p`.
3. Add a SQLite store and a reusable Bubble Tea BBS package.
4. Add a host `cmd/bbs` entrypoint.
5. Replace the current SSH transcript with the Bubble Tea BBS.
6. Validate host and guest flows, then publish the docs bundle.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
