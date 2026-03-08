---
Title: Diary
Ticket: QEMU-GO-INIT-002
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - web
    - networking
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T15:34:05.850448826-04:00
WhatFor: ""
WhenToUse: ""
---

# Diary

## Goal

This diary records the end-to-end implementation of userspace DHCP inside the Go `/init` binary, including the design rationale, commands run, validation results, and the commits used to land the work.

## Step 1: Create the DHCP ticket and capture the current failure mode

The existing QEMU demo already boots a statically linked Go `/init`, mounts the early filesystems, and serves the embedded webpage. The remaining failure is guest IPv4 configuration: the copied Ubuntu kernel boots and the Go server starts, but host-to-guest HTTP forwarding hangs because the guest never acquires an IP address.

I confirmed the root cause before creating the new ticket. The kernel config includes `CONFIG_VIRTIO_NET=y`, so the driver is present, but it does not include `CONFIG_IP_PNP`, so the kernel ignores `ip=dhcp` and passes it to userspace. That makes a userspace DHCP client in `/init` a valid next step if we want to avoid rebuilding the kernel.

### Prompt Context

**User prompt (verbatim):** "Ok, create a docmgr ticket to do that, add a detailed implementation guide, add tasks, and then work the tasks off one by one. 

Commit at appropriate intervals, keep a detailed diary all along.

Create a detailed analysis / design / implementation guide that is very detailed for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file
  references.
  It should be very clear and detailed. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Create a fresh ticket dedicated to userspace DHCP in the Go init runtime, plan the work formally, implement it incrementally with commits, and produce intern-grade documentation and delivery artifacts.

**Inferred user intent:** Keep the existing QEMU demo architecture, but remove the kernel-config dependency for DHCP by making the init binary self-contained enough to bring up guest networking on its own.

**Commit (code):** pending

### What I did

- Created ticket `QEMU-GO-INIT-002`.
- Added the design doc, diary, and implementation guide documents.
- Added a concrete task list and seeded the changelog.
- Carried forward the previously observed evidence that:
  - the guest boots,
  - the Go HTTP server starts,
  - `ip=dhcp` is ignored by the current kernel,
  - the host-forwarded port therefore hangs.

### Why

- The previous ticket documented the kernel-side fix path.
- This ticket needs to stand alone as the userspace-DHCP alternative, with its own design record and implementation diary.

### What worked

- `docmgr ticket create-ticket --ticket QEMU-GO-INIT-002 --title "Userspace DHCP in the Go init binary for QEMU guest networking" --topics go,qemu,linux,initramfs,web,networking`
- `docmgr doc add ...` for the design doc, diary, and implementation guide all succeeded.

### What didn't work

- N/A

### What I learned

- The next meaningful increment is not “more QEMU flags.” It is explicit interface setup in userspace because the kernel’s DHCP path is unavailable on this host kernel.

### What was tricky to build

- The planning has to preserve the existing repo structure and previous ticket while still creating a clean narrative for the new DHCP-specific workstream. That is why this ticket was created separately instead of overloading the previous one.

### What warrants a second pair of eyes

- The task ordering: interface discovery, DHCP, address/route programming, UI updates, and smoke validation need to land in a sequence that stays testable.

### What should be done in the future

- Convert the planning tasks into checked milestones as each commit lands.

### Code review instructions

- Start with `ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/tasks.md`.
- Then compare the failure analysis against the existing runtime in `cmd/init/main.go` and `internal/boot/boot.go`.

### Technical details

```bash
docmgr ticket create-ticket --ticket QEMU-GO-INIT-002 --title "Userspace DHCP in the Go init binary for QEMU guest networking" --topics go,qemu,linux,initramfs,web,networking
docmgr doc add --ticket QEMU-GO-INIT-002 --doc-type design-doc --title "Userspace DHCP architecture and implementation guide for the Go init runtime"
docmgr doc add --ticket QEMU-GO-INIT-002 --doc-type reference --title "Diary"
docmgr doc add --ticket QEMU-GO-INIT-002 --doc-type reference --title "Implementation guide"
```
