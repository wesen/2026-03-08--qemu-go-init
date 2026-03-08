---
Title: Persistent guest storage architecture and implementation plan for qemu-go-init
Ticket: QEMU-GO-INIT-005
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - ssh
    - web
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T17:42:23.891149207-04:00
WhatFor: ""
WhenToUse: ""
---

# Persistent guest storage architecture and implementation plan for qemu-go-init

## Executive Summary

The current `qemu-go-init` system boots a static Go binary from an initramfs and keeps almost all state in RAM. That is the right starting architecture for a tiny single-binary system, but it means every reboot discards SSH host keys, `authorized_keys`, app state, and any operator-created files. The correct next step is not to replace the root filesystem. It is to add a small persistent data disk and mount it early from PID 1 at a stable path such as `/var/lib/go-init`.

This ticket recommends a narrow persistence design:

- keep the initramfs root as the boot medium,
- attach one QEMU `virtio-blk` data disk,
- format it once as `ext4`,
- mount it from the Go PID 1 runtime after `/dev` is available,
- expose its mount status in the existing web status surface,
- store SSH host keys and app state under `/var/lib/go-init`.

That design preserves the strengths of the current system:

- minimal boot surface,
- no external init system,
- no dependency on a large guest userland,
- deterministic runtime ownership in Go.

## Problem Statement

The current system is intentionally ephemeral.

Evidence from the repo:

- [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go#L127) builds an archive that contains `/init`, `/dev`, `/proc`, `/sys`, and optional module files, but no durable writable disk.
- [internal/boot/boot.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/boot/boot.go#L29) mounts only `proc`, `sysfs`, and `devtmpfs`.
- [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go#L240) can write `/etc/resolv.conf`, which proves the in-memory root is writable, but those writes disappear after reboot.

This causes four practical problems:

1. Wish or any other SSH service must regenerate host keys on every boot.
2. There is no stable place to keep `authorized_keys`.
3. Application data and logs vanish across restarts.
4. Future features such as uploaded files, persistent settings, or random-seed carryover have nowhere durable to live.

The feature request is therefore not “implement a filesystem in Go.” The kernel already knows how to mount filesystems. The real requirement is to let the Go runtime provision and mount a durable guest volume and then treat it as the persistent application data root.

## Scope

In scope:

- architecture and implementation planning for one persistent guest data volume,
- early-boot device discovery and mount sequencing,
- data layout for SSH and application state,
- QEMU integration and smoke-test strategy,
- operator workflows for first boot and repeat boot.

Out of scope for this ticket:

- replacing the initramfs root with a full persistent root filesystem,
- multi-disk storage management,
- advanced partitioning,
- encryption,
- a general package-manager/userland story.

## Proposed Solution

### High-level model

Add a single QEMU-backed data disk and mount it inside the guest.

```text
Host repo
  |
  | creates/owns build/data.img
  v
QEMU virtio-blk device
  |
  v
Linux guest block device (/dev/vda or /dev/vdb)
  |
  v
Go PID 1 runtime
  |
  |- detect block device
  |- format on first boot if empty
  |- mount ext4 at /var/lib/go-init
  `- hand stable paths to higher-level services
```

### Recommended data layout

Use a single mount point:

- `/var/lib/go-init`

Suggested subdirectories:

- `/var/lib/go-init/ssh`
  - `ssh_host_ed25519`
  - `ssh_host_ed25519.pub`
  - `authorized_keys`
- `/var/lib/go-init/app`
  - application-specific durable data
- `/var/lib/go-init/log`
  - optional boot/session logs if the project later chooses to persist them
- `/var/lib/go-init/state`
  - future persistent random seed, checkpoints, or metadata

### Boot sequence changes

Today the runtime sequence in [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go#L15) is:

1. mount pseudo-filesystems,
2. load `virtio_rng`,
3. probe entropy,
4. configure networking,
5. start web UI.

The persistence-aware sequence should become:

1. mount pseudo-filesystems,
2. detect and mount persistent data volume,
3. load `virtio_rng`,
4. probe entropy,
5. configure networking,
6. start Wish and HTTP services using persistent paths.

### Why `ext4` on a QEMU data image

`ext4` is the correct default here because it is:

- simple,
- well understood,
- supported by host tooling,
- easy to validate from both the guest and the host,
- suitable for the tiny amount of state the project needs.

Using a single raw image file in `build/` also keeps the operator workflow simple:

- easy to create,
- easy to delete for a clean slate,
- easy to keep out of commits.

## Design Decisions

### Decision 1: Keep initramfs as the boot root

Reasoning:

- The current architecture is already working.
- Replacing the root filesystem would turn a focused persistence task into a full distro-building task.
- The single-binary mental model stays intact when the persistent disk is treated as an application data volume.

### Decision 2: Mount one application data directory, not a general writable root

Reasoning:

- SSH host keys and app state need durability, not a mutable root.
- A single mount point is easier to explain to a new intern.
- It sharply limits the blast radius of mount failures.

### Decision 3: Perform storage orchestration in Go PID 1

Reasoning:

- The repo’s defining feature is that boot logic is owned by the Go runtime.
- There is no external init system to delegate to.
- Storage status should become part of the same observable status model as networking and entropy.

### Decision 4: Prefer first-boot formatting only when explicitly safe

Reasoning:

- Automatically formatting the wrong block device would be catastrophic.
- The runtime should only format when it can clearly identify the intended data disk and when the filesystem signature is absent.
- The implementation should expose the chosen device, probe result, and formatting action in structured status.

## Alternatives Considered

### Alternative 1: Keep everything ephemeral

Rejected because:

- SSH host keys rotate every boot,
- users lose trust in the SSH endpoint,
- any real app state disappears.

### Alternative 2: Bake state files into the initramfs

Rejected because:

- that is not persistence,
- it requires rebuilding the archive to change state,
- it does not help with runtime-created files.

### Alternative 3: Replace the initramfs model with a full persistent root disk

Rejected for now because:

- it is a much larger architectural change,
- it complicates boot and recovery flows,
- it weakens the clarity of the single-binary demo.

### Alternative 4: Implement a userspace filesystem in Go

Rejected because:

- it is unnecessary,
- it shifts work away from the real problem,
- the kernel already knows how to mount standard filesystems.

## Implementation Plan

### Phase 1: Add storage configuration and QEMU disk plumbing

Files likely to change:

- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)

Tasks:

- add a `build/data.img` path,
- add size configuration such as `QEMU_DATA_IMAGE_SIZE ?= 64M`,
- attach the image with `virtio-blk`,
- decide whether smoke tests create/reset the image automatically.

### Phase 2: Add a storage package to the Go runtime

Recommended package:

- `internal/storage`

Suggested responsibilities:

- config loading,
- device discovery,
- filesystem signature probe,
- mount orchestration,
- result/status struct for the web UI.

Suggested status model:

```go
type Result struct {
    Enabled        bool   `json:"enabled"`
    Device         string `json:"device,omitempty"`
    MountPoint     string `json:"mountPoint,omitempty"`
    Filesystem     string `json:"filesystem,omitempty"`
    Formatted      bool   `json:"formatted"`
    Mounted        bool   `json:"mounted"`
    Step           string `json:"step,omitempty"`
    Error          string `json:"error,omitempty"`
}
```

### Phase 3: Add first-boot formatting logic

Important constraint:

- formatting must not happen blindly.

Recommended flow:

```text
if storage disabled:
    return disabled result

find candidate block device
if no device:
    return error result

if filesystem signature exists:
    skip format
else:
    run mkfs.ext4 equivalent path

mount device at /var/lib/go-init
mkdir required subdirectories
return structured result
```

Implementation note:

- If the repo wants to stay “no extra guest userland,” formatting may need to happen from the host side in the first implementation, or the project may need a small in-repo formatter strategy. The simplest first cut is often host-created and host-formatted disk images, with guest-side mount only.

### Phase 4: Move higher-level state onto the persistent volume

Expected changes:

- Wish host key path becomes `/var/lib/go-init/ssh/ssh_host_ed25519`
- `authorized_keys` becomes `/var/lib/go-init/ssh/authorized_keys`
- future app data lives under `/var/lib/go-init/app`

### Phase 5: Add observability and smoke validation

Changes:

- expose storage status in `/api/status`,
- add a storage panel in the web UI,
- write a reboot smoke test that proves host keys persist.

## Testing and Validation Strategy

### Unit tests

- config parsing,
- device-name selection,
- formatting guard logic,
- mountpoint path preparation.

### Integration tests

- create a fresh data image,
- boot guest,
- let runtime mount it,
- generate a Wish host key,
- reboot with the same image,
- verify the same host key fingerprint is observed.

### Operator validation commands

```bash
make initramfs
QEMU_DATA_IMAGE=build/data.img make run
ssh-keygen -lf known_host_key.pub
```

## Risks

- kernel may lack built-in support for the chosen block or filesystem path,
- host-side image creation can drift from guest expectations,
- automatic formatting can be dangerous if device selection is sloppy,
- persistence introduces “dirty state” failure modes that the current ephemeral system avoids.

## Open Questions

1. Should first-boot formatting happen in the guest or on the host?
2. Which block device name should the runtime consider authoritative?
3. Does the chosen kernel already have the needed built-in `virtio_blk` and `ext4` support?
4. Should logs remain ephemeral even after host keys and app state become persistent?

## References

- [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
- [internal/boot/boot.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/boot/boot.go)
- [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go)
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
- [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
