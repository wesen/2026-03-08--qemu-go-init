---
Title: Wish-based SSH app architecture, analysis, and implementation guide for the Go init runtime
Ticket: QEMU-GO-INIT-004
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - ssh
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: Makefile
      Note: current QEMU and initramfs run paths that need SSH port forwarding
    - Path: cmd/init/main.go
      Note: current PID 1 orchestration point where an SSH service would be added
    - Path: internal/networking/network.go
      Note: current userspace networking layer that must succeed before SSH is reachable
    - Path: internal/webui/site.go
      Note: existing status API that should surface SSH state during integration
    - Path: internal/webui/static/index.html
      Note: existing operator UI that should gain an SSH panel
    - Path: scripts/qemu-smoke.sh
      Note: current smoke harness that should grow SSH validation
    - Path: ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/go.mod
      Note: local probe module dependencies showing current Wish resolution
    - Path: ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/main.go
      Note: local Wish probe used to validate current API and PTY behavior
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T17:20:24.284257296-04:00
WhatFor: ""
WhenToUse: ""
---


# Wish-based SSH app architecture, analysis, and implementation guide for the Go init runtime

## Executive Summary

This ticket evaluates whether the current `qemu-go-init` system can host its own SSH service without carrying a separate SSH daemon such as OpenSSH or Dropbear in the guest. The conclusion is yes, with an important clarification: [Wish](https://github.com/charmbracelet/wish) is a strong fit for a self-hosted SSH application or TUI service, but it is not automatically a full OpenSSH replacement. It gives the repo an in-process SSH server, session middleware, and app-oriented session handling. General shell semantics, command execution policy, subsystem support, host key persistence, and authentication policy still need to be designed explicitly.

The current repo already has almost all the hard infrastructure needed for this integration:

- a statically linked Go PID 1 runtime in [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go),
- a rootless initramfs builder in [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go),
- QEMU automation in [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile) and [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh),
- and a status API plus webpage in [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go).

That means the SSH work should be treated as a new in-process service added to the existing runtime, not as a second operating environment. The recommended design is to add a new `internal/sshapp` package, start a Wish server after networking comes up, and keep the current web status surface during early iterations so the SSH path is inspectable from the host browser.

## Problem Statement

The current system proves that the guest can self-host a Go HTTP service as PID 1, but it does not yet provide a host-to-guest interactive control channel beyond the forwarded webpage. For projects that care about SSH-native workflows, browser access is not enough. The user wants to know whether the repo can self-host a “fun little SSH app” using Wish and avoid carrying a traditional SSH daemon or a more general userland just to get SSH connectivity.

There are three separate problems hidden inside that request:

1. Can the system self-host the SSH protocol server in-process?
2. Can it do so without OpenSSH or another guest daemon?
3. What exactly should “full SSH server” mean in this repo?

The research confirms the answer to the first two is yes. Wish creates a self-hosted SSH server in-process and can generate host keys on its own. The third question needs precise scoping:

- If “full SSH server” means “an SSH transport that accepts client connections and hosts custom sessions,” Wish fits well.
- If it means “a shell-compatible multi-user replacement for OpenSSH with the usual shell, exec, subsystem, and file transfer expectations,” Wish is only a starting point, not the entire answer.

That distinction matters because this repo currently has almost no guest userland beyond the Go binary itself. In a no-userland environment, the most natural Wish integration is a custom SSH app, not a general-purpose shell service.

## Current-State Analysis

### Current runtime shape

The current runtime entrypoint in [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go#L15) performs these boot steps:

1. start PID 1 child reaping,
2. mount `/proc`, `/sys`, and `/dev`,
3. load the `virtio_rng` kernel module,
4. probe entropy,
5. configure networking in userspace,
6. start the HTTP server and embedded web UI.

This is important because any SSH service would be inserted into that same orchestration sequence. The SSH server would not replace the boot process. It would be another long-lived service inside the same Go runtime.

### Current networking constraints

The networking package in [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go#L98) already selects an interface, brings it up, performs DHCP with a raw socket, writes `/etc/resolv.conf`, and returns a structured `Result` that is surfaced in the JSON API.

This means Wish does not need to solve networking. Its integration point is after the current networking setup succeeds.

### Current host/guest observability

The web status surface in [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go#L29) is already a useful operations console. It reports:

- mounts,
- networking,
- entropy,
- module-loading state.

That existing status model is a strong reason not to remove HTTP immediately when adding SSH. During early SSH iterations, the HTTP surface should stay in place and gain an `ssh` status section so failures remain browser-visible.

## Upstream Wish Research

The research used official package docs and a local compile-and-connect probe.

Observed upstream API from `go doc github.com/charmbracelet/wish` in the ticket probe module:

- `wish.NewServer(ops ...ssh.Option) (*ssh.Server, error)`
- `wish.WithAddress(addr string) ssh.Option`
- `wish.WithHostKeyPath(path string) ssh.Option`
- `wish.WithMiddleware(mw ...wish.Middleware) ssh.Option`
- `wish.WithAuthorizedKeys(path string) ssh.Option`
- `wish.WithPublicKeyAuth(h ssh.PublicKeyHandler) ssh.Option`
- `wish.WithSubsystem(key string, h ssh.SubsystemHandler) ssh.Option`

Key documented behavior:

- `wish.NewServer` creates a default SSH server and auto-generates a new ed25519 key pair if one does not exist.
- By default, the server accepts incoming password and public-key connections unless stricter auth options are configured.
- Middleware composes from first to last, with the last middleware executing first.

Important middleware findings from local docs:

- `activeterm.Middleware()` is explicitly “a middleware to block inactive PTYs.”
- `logging.Middleware()` provides connection-level logging.

## Local Probe Findings

The ticket includes an isolated experiment in:

- [scripts/wish-probe/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/main.go)
- [scripts/wish-probe/go.mod](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/go.mod)

The probe established these concrete facts:

1. Current module resolution used `github.com/charmbracelet/wish v1.4.7`.
2. The effective SSH package used by the server API was `github.com/charmbracelet/ssh`, not a direct `gliderlabs/ssh` import in the application code.
3. Starting a Wish server generated a host key file automatically:
   - `.wish_probe_ed25519`
   - `.wish_probe_ed25519.pub`
4. A local connection with `PreferredAuthentications=none` succeeded in the probe environment without external SSH userland in the server.
5. With `activeterm.Middleware()` enabled, a non-PTY exec-style session returned:

```text
Requires an active PTY
```

6. An interactive PTY-backed session succeeded and exposed PTY metadata.

That last point is especially important for design. If the target app is a Bubble Tea or other TUI-oriented SSH app, PTY enforcement is desirable. If the target app needs exec-style command sessions, `activeterm` should not be applied globally.

## Gap Analysis

The current repo has:

- networking,
- initramfs packaging,
- process lifetime management,
- QEMU port forwarding,
- status reporting.

It does not yet have:

- an SSH listener in the guest,
- guest-exposed host-key policy,
- an SSH auth policy,
- an SSH session model,
- QEMU forwarding for port 22 or another SSH port,
- host-side smoke validation for SSH.

The main technical gap is therefore not “can we do SSH at all?” It is “which SSH behavior do we actually want?”

## Proposed Solution

Add a Wish-hosted SSH application as another in-process service inside the current Go PID 1 runtime.

### High-level architecture

```text
Host
  |
  | ssh -p <forwarded-port> 127.0.0.1
  v
QEMU user networking hostfwd
  |
  v
Wish server inside Go PID 1 process
  |
  |- auth policy
  |- host key loading/generation
  |- middleware chain
  `- app session handler
```

### Recommended first product shape

The first implementation should be a custom interactive SSH app, not a full shell service.

Why:

- It matches Wish’s strengths.
- It avoids pretending the guest has a normal shell userland when it does not.
- It keeps the system “single binary” in a meaningful way.
- It makes the app logic explicit and reviewable.

### Recommended integration shape

Add a new package, tentatively:

- `internal/sshapp`

That package should own:

- config loading from env,
- SSH server startup,
- host key path policy,
- auth configuration,
- middleware composition,
- session handler logic,
- structured status reporting.

The top-level wiring in [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go) would then become conceptually:

```go
mounts := boot.PrepareFilesystem(logger)
moduleResult := kmod.LoadVirtioRNG(logger)
entropyResult := entropy.Probe(logger)
networkResult, err := networking.Configure(logger)
sshResult, sshServer, err := sshapp.NewServer(logger)
httpHandler, err := webui.NewHandler(...)

go sshapp.Serve(sshServer, logger)
go boot.ServeHTTP(addr, httpHandler, logger)

boot.WaitForever(logger)
```

The exact concurrency model can vary, but the key point is that Wish becomes just another managed server inside PID 1.

## Proposed API And Status Model

### Suggested runtime config

Add env vars such as:

- `GO_INIT_ENABLE_SSH=1`
- `GO_INIT_SSH_ADDR=:2222`
- `GO_INIT_SSH_HOST_KEY_PATH=/var/lib/go-init/ssh_host_ed25519`
- `GO_INIT_SSH_AUTHORIZED_KEYS=/etc/go-init/authorized_keys`
- `GO_INIT_SSH_REQUIRE_PTY=1`
- `GO_INIT_SSH_BANNER="..."` optional

Suggested design choice:

- default `GO_INIT_ENABLE_SSH=0` during the first integration patch,
- then switch default on once smoke coverage exists.

### Suggested status struct

Expose a new `ssh` section in `/api/status`, for example:

```json
{
  "ssh": {
    "enabled": true,
    "listenAddr": ":2222",
    "hostKeyPath": "/var/lib/go-init/ssh_host_ed25519",
    "hostKeyGenerated": true,
    "authMode": "authorized_keys",
    "requirePTY": true,
    "started": true,
    "error": ""
  }
}
```

This should be surfaced in:

- [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
- [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html)

## Design Decisions

### Decision 1: Use Wish, not a full external SSH daemon

Rationale:

- aligns with the single-binary goal,
- no OpenSSH or Dropbear packaging required,
- easier to keep boot/runtime logic inside the Go process.

### Decision 2: Start with a custom SSH app, not a shell replacement

Rationale:

- the guest does not currently provide a shell-oriented userland,
- a custom app fits the current “single Go binary” model,
- avoids misleading “full sshd” expectations too early.

### Decision 3: Keep the HTTP status surface during early SSH integration

Rationale:

- browser-visible debugging remains valuable,
- SSH failures are easier to inspect if the web status API is still alive.

### Decision 4: Use explicit auth configuration before enabling SSH by default

Rationale:

- the probe confirmed permissive defaults are convenient for experiments,
- but a production-like init runtime should not rely on accept-all behavior.

### Decision 5: Make PTY enforcement configurable

Rationale:

- PTY enforcement is correct for interactive TUI apps,
- but wrong for non-interactive command sessions.

## Alternatives Considered

### Add OpenSSH to the initramfs

Rejected because:

- it adds a separate daemon, config surface, and packaging burden,
- it weakens the “self-hosted in-process service” goal.

### Add Dropbear to the initramfs

Rejected for the same reason:

- still an external daemon and userland integration task,
- less aligned with the repo’s current architecture.

### Use `github.com/charmbracelet/ssh` directly instead of Wish

Possible, but not preferred initially.

Reason:

- Wish adds middleware patterns and app-oriented conveniences that match the user’s goal more closely.

### Replace the HTTP service with SSH entirely

Rejected for first implementation because:

- removing the web status surface would reduce observability during integration.

## Implementation Plan

### Phase 1: Introduce an SSH package

Add:

- `internal/sshapp/config.go`
- `internal/sshapp/server.go`
- `internal/sshapp/status.go`

Responsibilities:

- parse env config,
- build the Wish server,
- expose structured result/status.

### Phase 2: Add a minimal session app

Start with something deliberately small:

- greeting,
- session metadata,
- maybe a simple command menu or Bubble Tea view.

Avoid trying to emulate a shell first.

### Phase 3: Wire into PID 1 runtime

Update:

- [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)

Do this after networking succeeds and before the process settles into long-lived serving.

### Phase 4: Extend status API and UI

Update:

- [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
- [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html)

### Phase 5: Extend QEMU run paths

Update:

- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)

Add SSH host forwarding such as:

```text
hostfwd=tcp::2222-:2222
```

### Phase 6: Add host-side SSH smoke validation

Host validation should exercise:

- a non-PTY connection if supported,
- an interactive PTY connection if the app requires PTY,
- host key and auth behavior.

## Testing Strategy

### Local development probe

Re-run the ticket probe:

- [scripts/wish-probe/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/main.go)

This is the fastest iteration loop for Wish-specific behavior.

### Unit tests

Add tests for:

- config parsing,
- auth-mode selection,
- status serialization,
- middleware selection logic.

### End-to-end QEMU smoke test

Add a smoke mode that:

1. boots QEMU,
2. waits for the guest network,
3. connects with the host `ssh` client,
4. asserts expected output.

## Risks And Sharp Edges

### Risk 1: Host key persistence

If the SSH host key is generated inside a purely ephemeral initramfs, it will rotate on every boot. That is acceptable for experiments and annoying for real clients.

Mitigation:

- make host key persistence a deliberate design choice,
- support a stable on-disk path once writable guest storage exists.

### Risk 2: Authentication defaults

The Wish defaults are convenient for quick prototypes, but they are too permissive for a serious deployment.

Mitigation:

- require `authorized_keys` or a public-key callback before enabling SSH by default.

### Risk 3: PTY-only middleware

The probe showed `activeterm` blocks non-PTY sessions.

Mitigation:

- use it only for interactive app mode,
- or make it conditional.

### Risk 4: Product ambiguity

“Full SSH server” can mean too many things.

Mitigation:

- explicitly scope phase 1 as “custom self-hosted SSH app,”
- define later phases separately if shell/subsystem/file-transfer behavior is needed.

## Open Questions

- Should the first Wish app be purely interactive, or should it also support a simple command router for non-PTY exec sessions?
- Should SSH become a sibling to the HTTP service, or eventually replace it once the SSH app is mature enough?
- What authentication mode should be the repo default once the first implementation lands?
- Is host key persistence in scope for the first implementation, or a follow-up?

## References

Primary sources:

- Wish repository: https://github.com/charmbracelet/wish
- Wish package docs: https://pkg.go.dev/github.com/charmbracelet/wish
- `github.com/charmbracelet/ssh` package docs: https://pkg.go.dev/github.com/charmbracelet/ssh

Current repo files:

- [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
- [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go)
- [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
- [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html)

Ticket experiment files:

- [scripts/wish-probe/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/main.go)
- [scripts/wish-probe/go.mod](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/go.mod)

## Proposed Solution

<!-- Describe the proposed solution in detail -->

## Design Decisions

<!-- Document key design decisions and rationale -->

## Alternatives Considered

<!-- List alternative approaches that were considered and why they were rejected -->

## Implementation Plan

<!-- Outline the steps to implement this design -->

## Open Questions

<!-- List any unresolved questions or concerns -->

## References

<!-- Link to related documents, RFCs, or external resources -->
