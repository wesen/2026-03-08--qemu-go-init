---
Title: Diary
Ticket: QEMU-GO-INIT-008
Status: active
Topics:
    - qemu
    - go
    - ssh
    - bubbletea
    - pinocchio
    - debugging
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Recorded the AI-chat debug endpoints, validation steps, and the first concrete guest findings: missing config and missing CA roots."
LastUpdated: 2026-03-09T19:37:40Z
WhatFor: ""
WhenToUse: ""
---

# Diary

## Goal

Add enough observability around the Pinocchio-backed BBS chat mode to answer two concrete debugging questions from a running guest:

- What exact profile/config/runtime did the chat mode resolve?
- Can the guest reach the resolved provider over HTTPS, and if not, where does the failure happen?

## Context

The Bubble Tea BBS gained a Pinocchio chat mode in the previous ticket, but the chat surface appeared to hang after the user submitted a prompt. That left multiple plausible failure points:

- the shared-state mount might not contain the config file or profile registry the chat mode expects,
- the profile could resolve to the wrong API type, engine, or base URL,
- the API key could be missing after profile/config merging,
- the guest could lack outbound DNS, TCP, or TLS trust material.

The existing web UI only exposed coarse system status. It did not expose the Pinocchio runtime composition or any provider connectivity data.

## Quick Reference

New endpoints:

- `GET /api/debug/aichat/runtime`
  - Returns the resolved Pinocchio runtime for the BBS chat mode.
  - Includes:
    - guessed config home,
    - raw `config.yaml` contents if present,
    - raw profile registry contents,
    - resolved profile metadata,
    - effective `StepSettings`,
    - provider wiring including API keys and base URLs.
- `GET /api/debug/aichat/https-probe`
  - Performs a traced outbound HTTPS request using the same resolved provider/base URL/key selection as the chat mode.
  - Currently probes `GET <baseURL>/models`.
  - Includes:
    - request URL,
    - resolved provider/key selection,
    - DNS/connect/TLS timing events,
    - HTTP status and headers when present,
    - short response body preview,
    - terminal error if the request failed.

Code entry points:

- [debug.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/aichat/debug.go)
- [site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
- [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)

Validation commands used:

```bash
go test ./internal/aichat ./internal/webui ./cmd/init -count=1
make run KERNEL_IMAGE=qemu-vmlinuz QEMU_HOST_PORT=18086 QEMU_SSH_HOST_PORT=10028
curl http://127.0.0.1:18086/api/debug/aichat/runtime
curl http://127.0.0.1:18086/api/debug/aichat/https-probe
```

Key findings from the first live run:

- `config.yaml` was missing in the guest:
  - `stat /var/lib/go-init/shared/pinocchio/config.yaml: no such file or directory`
- the resolved provider was still `openai-responses` / `gpt-5-nano`
- the resolved `openai-api-key` was empty
- the TLS probe reached OpenAI and failed at certificate verification:
  - `x509: certificate signed by unknown authority`

## Usage Examples

Task 1: Add runtime introspection helpers.

- Added `DebugSnapshot(...)` to [debug.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/aichat/debug.go).
- This reuses the same runtime resolution flow as the live chat surface:
  - `resolveRuntime(...)`
  - `resolveBaseStepSettings(...)`
  - `profileswitch.NewManagerFromSources(...)`
  - `manager.Switch(profileSlug)`
- I refactored the shared resolution work into `loadRuntimeDetails(...)` so the chat surface and the debug endpoint do not drift.

Task 2: Expose HTTP endpoints.

- Added two handlers in [site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go):
  - `/api/debug/aichat/runtime`
  - `/api/debug/aichat/https-probe`
- Wired both from [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go) using the same BBS state root the SSH app uses.

Task 3: Validate and interpret the result.

- Unit validation passed:
  - `go test ./internal/aichat ./internal/webui ./cmd/init -count=1`
- Live guest validation passed structurally:
  - the VM booted,
  - the endpoints returned JSON,
  - the HTTPS probe reached the target provider.
- The runtime endpoint immediately exposed a configuration mismatch:
  - the guest had `profiles.yaml`,
  - the guest did not have `config.yaml`,
  - therefore the resolved `openai-api-key` was empty.
- The HTTPS probe then exposed a second independent issue:
  - DNS resolution succeeded,
  - TCP connect succeeded,
  - TLS handshake started,
  - certificate validation failed because the guest has no trusted CA roots available to the Go runtime.

Interpretation:

- The apparent “chat hang” was not just a UI problem.
- There are at least two guest-side prerequisites still missing for successful provider calls:
  - a usable Pinocchio config source containing credentials,
  - a CA trust store or other TLS-root strategy for outbound HTTPS.

## Related

- Ticket index: [index.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/09/QEMU-GO-INIT-008--debug-pinocchio-chat-connectivity-and-expose-profile-registry-inspection-endpoints/index.md)
- Tasks: [tasks.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/09/QEMU-GO-INIT-008--debug-pinocchio-chat-connectivity-and-expose-profile-registry-inspection-endpoints/tasks.md)
- Changelog: [changelog.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/09/QEMU-GO-INIT-008--debug-pinocchio-chat-connectivity-and-expose-profile-registry-inspection-endpoints/changelog.md)
