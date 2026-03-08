---
Title: Diary
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
LastUpdated: 2026-03-08T17:42:24.128760194-04:00
WhatFor: ""
WhenToUse: ""
---

# Diary

## Goal

Capture the design and planning work for adding a durable guest storage volume to the current initramfs-based runtime.

## Step 1: Create the persistence follow-up ticket and turn the storage question into an implementation plan

This step established a separate ticket for persistence rather than mixing storage design into the active Wish implementation ticket. That separation matters because storage changes touch boot order, QEMU invocation, filesystem policy, and data durability, while the current Wish work can still proceed against the existing ephemeral root.

The ticket is intentionally analysis-first. The user asked for a follow-up ticket with a proper implementation plan and analysis, not immediate storage code. I therefore treated this step as architecture mapping, problem framing, and execution planning for later work.

### Prompt Context

**User prompt (verbatim):** "Ok, let's create a follow up ticket to add proper persistence and store an implementation plan / analysis in there, but then go back to the current ticket, add tasks, and then implement task by task, committing at appropriate intervals, and keeping a detailed diary as you go."

**Assistant interpretation:** Create a new persistence-focused ticket with detailed design and implementation guidance, then return to the active Wish ticket to execute the SSH work in task-sized slices with commits and diary updates.

**Inferred user intent:** Separate future persistence architecture from the active SSH implementation so both tracks stay understandable and reviewable.

### What I did

- Created ticket `QEMU-GO-INIT-005`.
- Inspected the current boot/runtime files to anchor the storage plan to the real repo.
- Wrote the first persistence design doc and implementation guide.
- Converted the new ticket task list from placeholders into a real work queue.

### Why

- Persistent storage is a meaningful architectural addition and deserves its own searchable ticket trail.

### What worked

- The repo’s current boot boundaries were already clear enough to support a storage plan without guessing.

### What didn't work

- N/A

### What I learned

- The simplest useful persistence model is a mounted data volume, not a persistent root filesystem.

### What was tricky to build

- The tricky part was preserving the repo’s “single binary PID 1” identity while planning durability. That tension disappears once the persistent disk is framed as application state storage rather than as a replacement root.

### What warrants a second pair of eyes

- The eventual implementation should verify that the kernel/image combination has the needed built-in block and filesystem support before code is written against optimistic assumptions.

### What should be done in the future

- Implement the storage package and reboot persistence smoke coverage in this ticket, after the active Wish ticket lands.

### Code review instructions

- Start with:
  - [01-persistent-guest-storage-architecture-and-implementation-plan-for-qemu-go-init.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/design-doc/01-persistent-guest-storage-architecture-and-implementation-plan-for-qemu-go-init.md)
  - [02-implementation-guide.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/reference/02-implementation-guide.md)
- Compare the recommendations against:
  - [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
  - [internal/boot/boot.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/boot/boot.go)
  - [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)

### Technical details

Commands used:

```bash
sed -n '1,220p' cmd/init/main.go
sed -n '1,220p' internal/boot/boot.go
sed -n '1,260p' cmd/mkinitramfs/main.go
sed -n '1,260p' Makefile
sed -n '1,260p' scripts/qemu-smoke.sh
```

## Step 2: Validate and publish the persistence planning ticket

This step finished the follow-up ticket as a documentation artifact. The design and implementation plan were already written; the remaining work was to validate the metadata and publish the bundle so the persistence track is ready when implementation begins.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Make sure the follow-up persistence ticket is not just created, but validated and published like the other ticket work.

**Inferred user intent:** Leave the persistence plan in a reusable state that can be picked up later without redoing the analysis.

### What I did

- Ran `docmgr doctor --ticket QEMU-GO-INIT-005 --stale-after 30`.
- Ran a dry-run bundle upload.
- Uploaded the bundle to `/ai/2026/03/08/QEMU-GO-INIT-005`.
- Verified the remote listing.

### Why

- The persistence ticket is meant to be a real follow-up artifact, not a local scratch note.

### What worked

- `docmgr doctor --ticket QEMU-GO-INIT-005 --stale-after 30`
- `remarquee upload bundle --dry-run ...`
- `remarquee upload bundle ...`
- `remarquee cloud ls /ai/2026/03/08/QEMU-GO-INIT-005 --long --non-interactive`

### What didn't work

- N/A

### What I learned

- The persistence work is already documented enough to hand off even though no code has been started.

### What was tricky to build

- N/A

### What warrants a second pair of eyes

- The actual implementation ticket should confirm kernel support for `virtio_blk` and `ext4` before committing to the exact device/mount flow.

### What should be done in the future

- Implement the storage package and reboot persistence smoke path described in this ticket.

### Code review instructions

- Review:
  - [01-persistent-guest-storage-architecture-and-implementation-plan-for-qemu-go-init.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/design-doc/01-persistent-guest-storage-architecture-and-implementation-plan-for-qemu-go-init.md)
  - [02-implementation-guide.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/reference/02-implementation-guide.md)

### Technical details

Commands used:

```bash
docmgr doctor --ticket QEMU-GO-INIT-005 --stale-after 30
remarquee upload bundle --dry-run ...
remarquee upload bundle ...
remarquee cloud ls /ai/2026/03/08/QEMU-GO-INIT-005 --long --non-interactive
```
