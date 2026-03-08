---
Title: Research and prototype a Wish-based self-hosted SSH app for the Go init runtime
Ticket: QEMU-GO-INIT-004
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - ssh
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T17:20:18.308411543-04:00
WhatFor: ""
WhenToUse: ""
---

# Research and prototype a Wish-based self-hosted SSH app for the Go init runtime

## Overview

This ticket researches how to add a self-hosted SSH application to the existing Go PID 1 runtime using [Charmbracelet Wish](https://github.com/charmbracelet/wish), without depending on OpenSSH or another external SSH daemon inside the guest. It includes upstream API research, a local probe under `scripts/`, and a detailed implementation guide for integrating Wish into the current QEMU/initramfs system.

## Key Links

- [Design doc](./design-doc/01-wish-based-ssh-app-architecture-analysis-and-implementation-guide-for-the-go-init-runtime.md)
- [Implementation guide](./reference/02-implementation-guide.md)
- [Diary](./reference/01-diary.md)
- [Wish probe script](./scripts/wish-probe/main.go)

## Status

Current status: **active**

Current status detail: research complete, local probe complete, docs complete, and bundle uploaded to reMarkable.

## Topics

- go
- qemu
- linux
- initramfs
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
