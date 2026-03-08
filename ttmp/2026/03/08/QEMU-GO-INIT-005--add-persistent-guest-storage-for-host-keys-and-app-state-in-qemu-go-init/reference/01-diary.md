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

## Step 3: Add the guest-side storage package and keep it narrowly focused

This step implemented the first code slice for the persistence ticket: a dedicated `internal/storage` package that waits for a QEMU block device, mounts it as `ext4`, and prepares the durable directory layout under `/var/lib/go-init`. I intentionally kept the package focused on guest-side orchestration rather than mixing in host image creation or SSH-specific logic.

That narrow scope mattered because it let me validate the mount logic independently first. The new package has a small config surface, a structured status result, and unit tests for the parts that can be exercised off-guest, especially environment defaults and directory preparation.

### Prompt Context

**User prompt (verbatim):** "alright, close 004 and work on 005, create tasks, then work them off one by one, commit appropriately, keep a detailed diary."

**Assistant interpretation:** Close the completed Wish ticket, turn the persistence ticket into an implementation queue, and begin landing persistence code in small validated slices with commits and diary updates.

**Inferred user intent:** Move from persistence planning to real implementation without losing the disciplined ticket/commit trail used for the SSH ticket.

**Commit (code):** 2ffc837 — "Add guest storage mount package"

### What I did

- Added:
  - [internal/storage/storage.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/storage/storage.go)
  - [internal/storage/storage_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/storage/storage_test.go)
- Implemented:
  - storage config loading from env,
  - block-device wait logic,
  - `ext4` mount orchestration,
  - durable state-directory preparation for `ssh`, `app`, `log`, and `state`,
  - a structured storage status result for later API/UI exposure.

### Why

- The guest needs a single early-boot abstraction for persistence before higher-level services can depend on durable paths.

### What worked

- `gofmt -w internal/storage/storage.go internal/storage/storage_test.go`
- `go test ./internal/storage -count=1`

### What didn't work

- N/A

### What I learned

- The guest-side mount orchestration is much smaller than the surrounding QEMU/plumbing work. The real complexity comes from lifecycle and validation, not from the mount syscall itself.

### What was tricky to build

- The tricky part was deciding how much policy to encode up front. I kept formatting, labels, and host image preparation out of this first slice so the package would stay focused on “wait, mount, prepare directories.”

### What warrants a second pair of eyes

- The package assumes a single authoritative device path, currently `/dev/vda`. That is correct for the chosen QEMU drive layout, but it remains an important invariant.

### What should be done in the future

- Add the host-side image lifecycle and then wire the package into the actual boot flow.

### Code review instructions

- Review:
  - [storage.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/storage/storage.go)
  - [storage_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/storage/storage_test.go)
- Re-run:

```bash
go test ./internal/storage -count=1
```

### Technical details

Commands used:

```bash
gofmt -w internal/storage/storage.go internal/storage/storage_test.go
go test ./internal/storage -count=1
```

## Step 4: Add a reusable `data.img` workflow on the host side

This step gave the repo a concrete persistent disk artifact: `build/data.img`. The key design choice was create-once and reuse-on-reboot. Persistence does not matter if the image is recreated every time, so the `Makefile` now creates and formats the image only when it is missing, and both `make run` and `make smoke` attach the same raw file with `virtio-blk`.

### Prompt Context

**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Continue the persistence task list by wiring in the host-side artifact and QEMU drive attachment needed for a durable guest volume.

**Inferred user intent:** Make the persistence feature real in the run/smoke workflow, not just internally mountable in principle.

**Commit (code):** 4ec71f9 — "Add persistent data image workflow"

### What I did

- Updated [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile) to:
  - create `build/data.img`,
  - format it with `mkfs.ext4`,
  - attach it to QEMU as a `virtio` raw drive,
  - reuse the same image for later boots.
- Updated [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh) to require the data image when storage is enabled and include it in the QEMU argument list.

### Why

- The guest storage package needs an actual block device to mount, and persistence only matters if subsequent boots reuse the same disk image.

### What worked

- `bash -n scripts/qemu-smoke.sh`
- `make data-image QEMU_DATA_IMAGE=build/data.img QEMU_DATA_SIZE=64M`

### What didn't work

- N/A

### What I learned

- The kernel we are using already has the critical built-ins for this design:
  - `CONFIG_VIRTIO_BLK=y`
  - `CONFIG_EXT4_FS=y`

### What was tricky to build

- The tricky part was keeping the image lifecycle create-once rather than accidentally baking in a destructive “always format” behavior.

### What warrants a second pair of eyes

- `build/data.img` is intentionally an untracked build artifact. It must stay out of commits, because it is state, not source.

### What should be done in the future

- Mount the new disk during boot, expose its status, and prove that Wish state survives a reboot.

### Code review instructions

- Review:
  - [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
  - [qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
- Re-run:

```bash
bash -n scripts/qemu-smoke.sh
make data-image QEMU_DATA_IMAGE=build/data.img QEMU_DATA_SIZE=64M
```

### Technical details

Commands used:

```bash
bash -n scripts/qemu-smoke.sh
make data-image QEMU_DATA_IMAGE=build/data.img QEMU_DATA_SIZE=64M
```

## Step 5: Mount storage during boot and expose it through the operator surfaces

This step integrated the storage layer into the live guest. The boot sequence now mounts the persistent disk before starting the rest of the runtime, the existing Wish host-key path naturally resolves onto the mounted volume, and the browser/API status surfaces gained a `storage` section so operators can see whether the mount succeeded and which directories were prepared.

### Prompt Context

**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Keep working the task list by connecting the guest storage package to real boot and observability paths.

**Inferred user intent:** Make persistence a first-class runtime capability, not just a background helper package.

### What I did

- Updated [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go) to mount storage during boot and fail fast if storage preparation fails.
- Updated [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go) and [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html) to expose and render storage state.
- Confirmed that the default Wish host-key path `/var/lib/go-init/ssh/ssh_host_ed25519` now resolves onto the mounted volume.

### Why

- Without boot integration and observability, the mounted disk would still be a hidden implementation detail rather than a usable system capability.

### What worked

- `gofmt -w cmd/init/main.go internal/webui/site.go`
- `go test ./... -count=1`

### What didn't work

- The first end-to-end persistence run accidentally hit an older QEMU instance still bound to the default forwarded ports, which made the returned status JSON misleading. The giveaway was an old uptime and the absence of the new `storage` field. Re-running on fresh ports fixed that and confirmed the new guest was being exercised.

### What I learned

- The status UI remains the fastest debugging tool in this repo. The missing `storage` field immediately exposed that I was talking to the wrong VM.

### What was tricky to build

- The tricky part was not the actual boot wiring; it was making sure the end-to-end test environment was hitting the newly built guest and not a stale process on the same host ports.

### What warrants a second pair of eyes

- Storage is now a boot-critical dependency when enabled. That is intentional, but it means future runs must keep the drive layout and device path stable.

### What should be done in the future

- Add reboot-aware smoke validation and verify the persisted SSH host key survives an actual second boot.

### Code review instructions

- Review:
  - [main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
  - [site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
  - [index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html)
- Re-run:

```bash
go test ./... -count=1
```

### Technical details

Commands used:

```bash
gofmt -w cmd/init/main.go internal/webui/site.go
go test ./... -count=1
```

## Step 6: Debug zero-byte persisted host keys and harden the SSH write path

This was the most important debugging step in the ticket. The first reboot attempt failed with:

```text
fatal: start ssh app: create wish server: ssh: no key found
```

The persistent image inspection showed why. The guest had created `/var/lib/go-init/ssh/ssh_host_ed25519` and `.pub`, but both files were zero bytes inside `build/data.img`. That means the path existed, but it did not contain a parseable private key on the next boot. Since Wish’s `WithHostKeyPath` only generates a key when the file is missing, the second boot failed immediately when it tried to load the empty file.

I fixed that by taking ownership of host-key durability in our code. Instead of relying on Wish/keygen to both generate and persist the first boot’s key, the repo now validates the file, repairs missing or zero-byte keys, and writes the private/public key pair atomically before starting Wish.

### Prompt Context

**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Continue implementing the persistence ticket and resolve the real reboot failure instead of papering over it.

**Inferred user intent:** Make persistence genuinely correct, not just superficially working on first boot.

**Commit (code):** 90a3439 — "Persist SSH host keys across guest reboot"

### What I did

- Inspected `build/data.img` with `debugfs` and confirmed the persisted host-key files were zero bytes.
- Read the Wish/keygen implementation to confirm `WithHostKeyPath` only generates on missing files and then delegates to `ssh.HostKeyFile`.
- Updated [internal/sshapp/server.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/server.go) to:
  - validate the existing host key,
  - repair missing or zero-byte keys,
  - generate Ed25519 keys in-process,
  - write the private and public key atomically,
  - `fsync` the files and containing directory before continuing.
- Added [hostkey_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/hostkey_test.go) covering both fresh creation and zero-byte repair.
- Reworked [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh) into a two-boot flow that:
  - boots the guest,
  - verifies storage and SSH,
  - scans the host key,
  - reboots with the same image,
  - scans again and compares fingerprints.

### Why

- A persistent storage feature is not complete unless the data actually survives a reboot in a form the next boot can use.

### What worked

- `go test ./internal/sshapp -count=1`
- `go test ./... -count=1`
- `make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18082 QEMU_SSH_HOST_PORT=10024`

Observed final proof:

```text
first host key:  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPYE76vySKUyC04XllOwgN2WDFz9K1Ys4N3KNMzfgevR
second host key: ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPYE76vySKUyC04XllOwgN2WDFz9K1Ys4N3KNMzfgevR
```

### What didn't work

- The first failed reboot attempt produced:

```text
fatal: start ssh app: create wish server: ssh: no key found
```

- Image inspection showed:

```text
debugfs -R 'ls -l /ssh' build/data.img
...
ssh_host_ed25519       0
ssh_host_ed25519.pub   0
```

### What I learned

- “File exists” is not enough for persisted credentials. The startup path has to validate the file contents and not just the pathname.
- For an environment that may be power-cut by the smoke harness, atomic write plus directory sync is worth doing explicitly.

### What was tricky to build

- The tricky part was distinguishing between a storage bug and a key-generation bug. The mounted filesystem was working correctly; the actual failure was that the first-write persistence path was not robust enough against an abrupt shutdown, leaving zero-byte files that later looked “present” but unusable.

### What warrants a second pair of eyes

- The current smoke harness still stops QEMU by killing the process. The host-key fix makes that safe enough for this workload, but a future graceful shutdown path would still be cleaner for other classes of persistent data.

### What should be done in the future

- Finalize the ticket bookkeeping, rerun `docmgr doctor`, and refresh the published bundle.
- The next natural follow-up after this ticket is public-key auth with a persistent `authorized_keys` file.

### Code review instructions

- Start with:
  - [server.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/server.go)
  - [hostkey_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/hostkey_test.go)
  - [qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
- Re-run:

```bash
go test ./internal/sshapp -count=1
go test ./... -count=1
make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18082 QEMU_SSH_HOST_PORT=10024
```

### Technical details

Commands used:

```bash
debugfs -R 'ls -l /ssh' build/data.img
debugfs -R 'stat /ssh/ssh_host_ed25519' build/data.img
go test ./internal/sshapp -count=1
go test ./... -count=1
make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18082 QEMU_SSH_HOST_PORT=10024
```

## Step 7: Validate the completed persistence ticket and refresh the published bundles

This final step turned the implementation into a finished ticket artifact. At this point the code and the two-boot proof were already done; the remaining work was to normalize the ticket metadata, rerun `docmgr doctor`, refresh the published bundles for both the newly completed persistence ticket and the now-closed Wish ticket, and leave the task list in a clean finished state.

### Prompt Context

**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Close out the persistence implementation with the same rigor as the earlier ticket work: validated docs, updated diary, and refreshed reMarkable output.

**Inferred user intent:** End with a complete, reviewable, published implementation artifact rather than stopping at working code.

### What I did

- Updated ticket `004` to use the valid `complete` status vocabulary.
- Marked ticket `005` complete and checked off the final task.
- Re-ran:
  - `docmgr doctor --ticket QEMU-GO-INIT-004 --stale-after 30`
  - `docmgr doctor --ticket QEMU-GO-INIT-005 --stale-after 30`
- Force-uploaded the refreshed bundles to:
  - `/ai/2026/03/08/QEMU-GO-INIT-004`
  - `/ai/2026/03/08/QEMU-GO-INIT-005`
- Verified the remote listings.

### Why

- The ticket only becomes useful long-term once the implemented state is reflected in the searchable docs and the off-repo bundle.

### What worked

- `docmgr doctor --ticket QEMU-GO-INIT-004 --stale-after 30`
- `docmgr doctor --ticket QEMU-GO-INIT-005 --stale-after 30`
- `remarquee upload bundle --force ...`
- `remarquee cloud ls /ai/2026/03/08/QEMU-GO-INIT-004 --long --non-interactive`
- `remarquee cloud ls /ai/2026/03/08/QEMU-GO-INIT-005 --long --non-interactive`

### What didn't work

- `docmgr doctor` initially warned that ticket `004` used `completed`, but the vocabulary expects `complete`. Changing the frontmatter value resolved that cleanly.

### What I learned

- Small metadata mismatches are worth fixing immediately. They are easy to miss during rapid implementation, but they matter for searchability and tooling cleanliness later.

### What was tricky to build

- The only tricky part here was remembering that the ticket had evolved from “plan” to “implemented” and that the published bundles needed to be force-refreshed rather than left as older snapshots.

### What warrants a second pair of eyes

- The persistence ticket is complete for its scope, but the next obvious security step is still to move from `authMode: "none"` to persistent `authorized_keys`-based public-key auth.

### What should be done in the future

- Build the next ticket around SSH authentication hardening and persistent `authorized_keys`.

### Code review instructions

- Review:
  - [index.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/index.md)
  - [tasks.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/tasks.md)
  - [changelog.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/changelog.md)

### Technical details

Commands used:

```bash
docmgr doctor --ticket QEMU-GO-INIT-004 --stale-after 30
docmgr doctor --ticket QEMU-GO-INIT-005 --stale-after 30
remarquee upload bundle --force ...
remarquee cloud ls /ai/2026/03/08/QEMU-GO-INIT-004 --long --non-interactive
remarquee cloud ls /ai/2026/03/08/QEMU-GO-INIT-005 --long --non-interactive
```
