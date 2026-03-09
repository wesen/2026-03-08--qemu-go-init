---
Title: Debug Pinocchio chat connectivity and expose profile registry inspection endpoints
Ticket: QEMU-GO-INIT-008
Status: active
Topics:
    - qemu
    - go
    - ssh
    - bubbletea
    - pinocchio
    - debugging
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Added live AI-chat debug endpoints, then fixed guest config sourcing, CA trust, log noise, and backend error propagation so chat failures can render visibly in the TUI."
LastUpdated: 2026-03-09T20:03:00Z
WhatFor: ""
WhenToUse: ""
---

# Debug Pinocchio chat connectivity and expose profile registry inspection endpoints

## Overview

This ticket added guest-visible debugging surfaces for the Bubble Tea BBS chat mode so we can inspect the exact Pinocchio runtime the SSH chat path uses and test outbound HTTPS from inside the VM. The current implementation is complete and the first live validation already narrowed the failure: the guest did not have a `pinocchio/config.yaml`, so the resolved OpenAI API key was empty, and an outbound probe to `api.openai.com` failed certificate verification because the guest does not yet provide a CA trust store.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

Latest findings:

- The BBS chat path still resolves `gpt-5-nano` from the shared `profiles.yaml`.
- The guest now sees `/var/lib/go-init/shared/pinocchio/config.yaml` and resolves the expected `openai-api-key`.
- The guest now verifies provider TLS successfully; `/api/debug/aichat/https-probe` returns `200 OK`.
- Pinocchio's backend now returns `ErrorMsg` on inference failures so the TUI can show an explicit error instead of only logging and silently finishing.

## Topics

- qemu
- go
- ssh
- bubbletea
- pinocchio
- debugging

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
