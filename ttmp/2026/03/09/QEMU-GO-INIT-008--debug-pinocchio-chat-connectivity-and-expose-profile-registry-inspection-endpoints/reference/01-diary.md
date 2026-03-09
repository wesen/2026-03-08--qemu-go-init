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
Summary: "Recorded the AI-chat debug endpoints, the initial guest failures, and the second slice that fixed guest config sourcing, CA roots, log reduction, and visible UI error propagation."
LastUpdated: 2026-03-09T20:03:00Z
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

Continuation plan:

- Fix host config sourcing.
  - The current Makefile copies `config.yaml` from `$(HOME)/.config/pinocchio/config.yaml`.
  - The active user setup appears to keep the config at `~/.pinocchio/config.yaml` while `profiles.yaml` still lives at `~/.config/pinocchio/profiles.yaml`.
  - The next implementation slice should stop assuming a single directory for both files.
- Add CA roots to the guest.
  - The most pragmatic change is to bake a PEM bundle such as `/etc/ssl/certs/ca-certificates.crt` into the initramfs and, if needed, set `SSL_CERT_FILE` for the guest runtime.
- Reduce terminal log noise.
  - The provider failure is currently visible in logs, but the Bobatea/Pinocchio trace flood makes the TUI harder to inspect while testing.
  - The next slice should either lower the log level or redirect those logs away from the interactive SSH session.
- Render backend failures explicitly.
  - The current backend logs the provider error and still returns `BackendFinishedMsg`.
  - The next slice should emit an error message that Bobatea can render as `StateError` or as a visible assistant/timeline failure entry.

Follow-up implementation results:

- Host config sourcing:
  - Updated [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile) to split `PINOCCHIO_HOST_CONFIG_FILE` and `PINOCCHIO_HOST_PROFILES_FILE`.
  - Default config lookup now prefers `~/.pinocchio/config.yaml` if it exists.
  - `profiles.yaml` continues to default to `~/.config/pinocchio/profiles.yaml`.
  - The shared-state sync now removes stale guest copies before copying the current host files.
- Guest TLS trust:
  - Extended [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go) with generic `-file-map` support for regular files in addition to `-module-map`.
  - Updated [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile) to bake `/etc/ssl/certs/ca-certificates.crt` into the initramfs.
  - Updated [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go) to set `SSL_CERT_FILE` to that bundle path when present.
- Log reduction:
  - Added [config.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/zlog/config.go).
  - Both [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go) and [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/bbs/main.go) now set the default zerolog level to `warn`, overridable with `GO_INIT_ZEROLOG_LEVEL`.
- Visible UI error handling:
  - Updated Pinocchio's [backend.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/ui/backend.go) so `handle.Wait()` failures return `boba_chat.ErrorMsg(err)`.
  - Added [backend_test.go](/home/manuel/code/wesen/corporate-headquarters/pinocchio/pkg/ui/backend_test.go) to lock that behavior in.
  - Updated Bobatea's [model.go](/home/manuel/code/wesen/corporate-headquarters/bobatea/pkg/chat/model.go) so entering error state recomputes layout and dismissing an error refocuses input.

Validation after follow-up work:

- `go test ./cmd/init ./cmd/bbs ./cmd/mkinitramfs ./internal/aichat ./internal/webui ./internal/zlog -count=1`
- `go test ./pkg/ui -count=1` in the Pinocchio repo
- `go test ./pkg/chat -count=1` in the Bobatea repo
- live guest revalidation:
  - `/api/debug/aichat/runtime` showed `config.yaml` present with the expected layered OpenAI defaults
  - `/api/debug/aichat/https-probe` returned `200 OK`

## Related

- Ticket index: [index.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/09/QEMU-GO-INIT-008--debug-pinocchio-chat-connectivity-and-expose-profile-registry-inspection-endpoints/index.md)
- Tasks: [tasks.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/09/QEMU-GO-INIT-008--debug-pinocchio-chat-connectivity-and-expose-profile-registry-inspection-endpoints/tasks.md)
- Changelog: [changelog.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/09/QEMU-GO-INIT-008--debug-pinocchio-chat-connectivity-and-expose-profile-registry-inspection-endpoints/changelog.md)
