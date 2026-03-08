---
Title: Implementation guide
Ticket: QEMU-GO-INIT-005
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - ssh
    - web
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T17:42:24.367311866-04:00
WhatFor: ""
WhenToUse: ""
---

# Implementation guide

## Goal

Give a new engineer a practical map for implementing persistent guest storage without breaking the current initramfs-first design.

## Context

The current system already proves that a Go binary can be PID 1, mount pseudo-filesystems, bring up networking, and host services inside QEMU. The persistence work should extend that system, not replace it.

Use this guide when you need to:

- add a durable guest data volume,
- move SSH keys or app state onto that volume,
- validate that data survives a reboot.

## Quick Reference

### Proposed runtime paths

| Purpose | Path |
| --- | --- |
| persistent mount root | `/var/lib/go-init` |
| Wish host keys | `/var/lib/go-init/ssh/ssh_host_ed25519` |
| `authorized_keys` | `/var/lib/go-init/ssh/authorized_keys` |
| app state | `/var/lib/go-init/app` |
| optional logs | `/var/lib/go-init/log` |

### Proposed package layout

| File | Responsibility |
| --- | --- |
| `internal/storage/config.go` | environment variables and defaults |
| `internal/storage/storage.go` | orchestration entrypoint |
| `internal/storage/probe.go` | device/filesystem discovery |
| `internal/storage/mount_linux.go` | Linux mount calls |
| `internal/storage/storage_test.go` | config and decision tests |

### Suggested environment variables

```text
GO_INIT_ENABLE_STORAGE=1
GO_INIT_STORAGE_DEVICE=/dev/vda
GO_INIT_STORAGE_MOUNT_POINT=/var/lib/go-init
GO_INIT_STORAGE_FILESYSTEM=ext4
GO_INIT_STORAGE_FORMAT_IF_EMPTY=0
```

### Suggested pseudocode

```go
cfg := storage.LoadConfigFromEnv()
result, err := storage.Prepare(logger, cfg)
if err != nil {
    logger.Printf("fatal: prepare storage: %v", err)
    boot.Halt(logger)
}

sshCfg.HostKeyPath = filepath.Join(result.MountPoint, "ssh", "ssh_host_ed25519")
```

## Usage Examples

### Example: host-side image preparation

```bash
truncate -s 64M build/data.img
mkfs.ext4 -F build/data.img
```

### Example: attach the image to QEMU

```bash
qemu-system-x86_64 \
  -drive file=build/data.img,if=virtio,format=raw
```

### Example: guest-side boot flow

```text
PrepareFilesystem()
PreparePersistentStorage()
LoadVirtioRNG()
ConfigureNetworking()
StartWish()
ServeHTTP()
```

### Example: reboot persistence validation

1. Boot with a fresh data image.
2. Let Wish generate `ssh_host_ed25519`.
3. Record the SSH host key fingerprint from the host.
4. Reboot the guest with the same image.
5. Confirm the fingerprint matches.

## Related

- [01-persistent-guest-storage-architecture-and-implementation-plan-for-qemu-go-init.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/design-doc/01-persistent-guest-storage-architecture-and-implementation-plan-for-qemu-go-init.md)
- [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
- [internal/boot/boot.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/boot/boot.go)
