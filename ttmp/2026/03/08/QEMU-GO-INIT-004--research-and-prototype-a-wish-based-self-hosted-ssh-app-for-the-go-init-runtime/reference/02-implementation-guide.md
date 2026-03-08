---
Title: Implementation guide
Ticket: QEMU-GO-INIT-004
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - ssh
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: Makefile
      Note: build and run knobs that will need SSH forwarding and config
    - Path: cmd/init/main.go
      Note: file-level integration point for the future SSH service
    - Path: scripts/qemu-smoke.sh
      Note: future host-side SSH smoke validation harness
    - Path: ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/main.go
      Note: copyable experiment entrypoint for validating Wish behavior
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T17:20:24.899413667-04:00
WhatFor: ""
WhenToUse: ""
---


# Implementation guide

## Goal

Provide a copy/paste-ready implementation plan for adding a Wish-based SSH app to the current Go PID 1 runtime.

## Context

The current repo already boots a static Go `/init`, configures networking, and serves an HTTP status page. This guide describes the shortest practical path to adding a self-hosted SSH application without OpenSSH or another guest daemon.

## Quick Reference

### Recommended package layout

```text
internal/sshapp/
  config.go
  server.go
  status.go
  session.go
  server_test.go
```

### Suggested runtime env vars

```text
GO_INIT_ENABLE_SSH=1
GO_INIT_SSH_ADDR=:2222
GO_INIT_SSH_HOST_KEY_PATH=/var/lib/go-init/ssh_host_ed25519
GO_INIT_SSH_AUTHORIZED_KEYS=/etc/go-init/authorized_keys
GO_INIT_SSH_REQUIRE_PTY=1
```

### Suggested status struct

```go
type Result struct {
    Enabled           bool   `json:"enabled"`
    ListenAddr        string `json:"listenAddr,omitempty"`
    HostKeyPath       string `json:"hostKeyPath,omitempty"`
    HostKeyGenerated  bool   `json:"hostKeyGenerated,omitempty"`
    AuthMode          string `json:"authMode,omitempty"`
    RequirePTY        bool   `json:"requirePTY,omitempty"`
    Started           bool   `json:"started"`
    Error             string `json:"error,omitempty"`
}
```

### Suggested Wish server construction

```go
server, err := wish.NewServer(
    wish.WithAddress(cfg.ListenAddr),
    wish.WithHostKeyPath(cfg.HostKeyPath),
    wish.WithAuthorizedKeys(cfg.AuthorizedKeysPath),
    wish.WithMiddleware(
        logging.Middleware(),
        maybeActiveTerm(cfg.RequirePTY),
        sessionMiddleware(logger, cfg),
    ),
)
```

### Suggested `cmd/init` wiring

```go
networkResult, err := networking.Configure(logger)
if err != nil { ... }

sshResult, sshServer, err := sshapp.Build(logger)
if err != nil { ... }

handler, err := webui.NewHandler(webui.Options{
    ListenAddr: addr,
    Mounts:     mounts,
    Network:    networkResult,
    SSH:        sshResult,
})

go sshapp.Serve(sshServer, logger)
err = boot.ServeHTTP(addr, handler, logger)
```

### Host-side smoke validation sketch

```bash
timeout 90s env KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 QEMU_SSH_HOST_PORT=2222 make smoke
ssh -tt -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p 2222 localhost
```

## Usage Examples

### Re-run the local Wish probe

```bash
cd ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe
go run .
```

In another terminal:

```bash
ssh -tt \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o PreferredAuthentications=none \
  -o PubkeyAuthentication=no \
  -o PasswordAuthentication=no \
  -p 22230 localhost
```

### Probe non-PTY behavior

```bash
ssh \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o PreferredAuthentications=none \
  -o PubkeyAuthentication=no \
  -o PasswordAuthentication=no \
  -p 22230 localhost true
```

With `activeterm.Middleware()` enabled, the observed result was:

```text
Requires an active PTY
```

That is the simplest proof that PTY-only middleware should be a deliberate choice.

### File checklist for the real repo integration

1. Add `internal/sshapp/*`
2. Update [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
3. Update [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
4. Update [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html)
5. Update [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
6. Update [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)

## Related

- [01-wish-based-ssh-app-architecture-analysis-and-implementation-guide-for-the-go-init-runtime.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/design-doc/01-wish-based-ssh-app-architecture-analysis-and-implementation-guide-for-the-go-init-runtime.md)
- [01-diary.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/reference/01-diary.md)
- [scripts/wish-probe/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/main.go)
