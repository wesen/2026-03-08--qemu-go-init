---
Title: Add explicit early-boot entropy support and diagnostics for the QEMU Go init runtime
Ticket: QEMU-GO-INIT-003
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - web
    - networking
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T16:34:44.539644361-04:00
WhatFor: ""
WhenToUse: ""
---

# Add explicit early-boot entropy support and diagnostics for the QEMU Go init runtime

## Overview

This ticket now implements explicit early-boot entropy support for the demo guest. QEMU exposes `virtio-rng`, the initramfs carries a matching `virtio_rng` kernel module, the Go PID 1 runtime loads that module during boot, and the status API plus embedded webpage report the resulting entropy state.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

The code work is complete, `docmgr doctor` passes cleanly, and the ticket bundle is uploaded to reMarkable at `/ai/2026/03/08/QEMU-GO-INIT-003`.

## Topics

- go
- qemu
- linux
- initramfs
- web
- networking

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
