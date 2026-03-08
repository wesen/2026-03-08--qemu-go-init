---
Title: Add persistent guest storage for host keys and app state in qemu-go-init
Ticket: QEMU-GO-INIT-005
Status: complete
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - ssh
    - web
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T17:42:23.623957473-04:00
WhatFor: ""
WhenToUse: ""
---

# Add persistent guest storage for host keys and app state in qemu-go-init

## Overview

This ticket captures the follow-up design work for giving the tiny QEMU guest a real persistent data volume. The current system boots entirely from initramfs, which is enough for HTTP, DHCP, and an in-process SSH app, but not enough for stable host keys, durable `authorized_keys`, reusable app data, or any workflow that expects files to survive a reboot.

The goal here is not to replace the initramfs boot model. The goal is to keep the existing single-binary PID 1 design and add one mounted persistent data volume that the Go runtime can manage from early boot. The design work in this ticket explains the storage model, the boot-time mount sequence, the data layout, validation strategy, and the risks that matter before implementation starts.

## Key Links

- [Design doc](./design-doc/01-persistent-guest-storage-architecture-and-implementation-plan-for-qemu-go-init.md)
- [Implementation guide](./reference/02-implementation-guide.md)
- [Diary](./reference/01-diary.md)

## Status

Current status: **complete**

Current status detail: persistent storage is implemented, validated with a two-boot QEMU smoke test, and the refreshed bundle is uploaded to reMarkable.

## Topics

- go
- qemu
- linux
- initramfs
- ssh
- web

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
