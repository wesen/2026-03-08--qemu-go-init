---
Title: Migrate guest chat persistence and logging to upstream CGO-backed SQLite stores
Ticket: QEMU-GO-INIT-009
Status: active
Topics:
    - go
    - qemu
    - sqlite
    - pinocchio
    - ssh
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-09T16:22:28.31991848-04:00
WhatFor: "Track the migration from the current pure-Go guest runtime to a dynamically packaged CGO guest runtime that can reuse upstream Pinocchio SQLite turn and timeline stores and persist runtime logs."
WhenToUse: "Use when implementing or reviewing CGO runtime packaging, SQLite-backed chat persistence, and host or guest log capture in qemu-go-init."
---

# Migrate guest chat persistence and logging to upstream CGO-backed SQLite stores

## Overview

This ticket migrates the guest runtime from the current pure-Go SQLite setup, which only stores BBS posts, to a CGO-backed runtime that can directly reuse the upstream Pinocchio SQLite turn and timeline stores. It also adds durable application logging and a host-side path for capturing QEMU serial logs into persistent storage.

The goal is not just “store more things in SQLite.” The goal is to align the guest runtime with the same persistence model already used upstream, while preserving the qemu-go-init product shape: boot a tiny initramfs, bring up networking, expose the BBS and chat over SSH, and keep the whole environment debuggable.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

Current slice plan:

1. Document and prove the dynamic CGO packaging model.
2. Make `/init` boot as a dynamically linked ELF from initramfs.
3. Reuse upstream Pinocchio turn and timeline SQLite stores inside the guest.
4. Persist guest logs to SQLite and expose their status via HTTP.
5. Capture host-side QEMU serial logs and import them into SQLite from the host side.

## Topics

- go
- qemu
- sqlite
- pinocchio
- ssh

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

## Primary Deliverables

- [01-cgo-backed-sqlite-persistence-and-runtime-packaging-plan-for-qemu-go-init.md](./design-doc/01-cgo-backed-sqlite-persistence-and-runtime-packaging-plan-for-qemu-go-init.md)
- [02-implementation-guide.md](./reference/02-implementation-guide.md)
- [01-diary.md](./reference/01-diary.md)
