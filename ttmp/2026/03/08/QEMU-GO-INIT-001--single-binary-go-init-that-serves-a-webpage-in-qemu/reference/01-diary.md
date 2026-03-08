---
Title: Diary
Ticket: QEMU-GO-INIT-001
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - web
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/init/main.go
      Note: Implemented_PID_1_demo
    - Path: internal/initramfs/writer.go
      Note: Implemented_rootless_archive_writer
    - Path: internal/initramfs/writer_test.go
      Note: Validation_for_newc_archive_layout
    - Path: scripts/qemu-smoke.sh
      Note: Documented_smoke_test_failure_mode
    - Path: ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/sources/local/qemu-go-guide.md
      Note: Original_user_supplied_guidance
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T14:14:54.368481079-04:00
WhatFor: ""
WhenToUse: ""
---


# Diary

## Goal

This diary captures the implementation work for `QEMU-GO-INIT-001`: ticket setup, source import, the initial code scaffold, validation results, documentation, and delivery evidence.

## Step 1: Initialize the ticket and import the external guide

The repository started as an empty Git working tree with only `.git` present. That meant the documentation workspace, the ticket, and the implementation all had to be created from scratch before any code could be written.

The first goal was to formalize the work in `docmgr` and capture the external source material exactly once in the ticket. Doing that early ensured the later design doc could point at the imported guide instead of relying on memory or ad hoc notes.

### Prompt Context

**User prompt (verbatim):** "Create a new docmgr ticket to create a single binary go init process that serves a webpage using qemu. IMport the file /tmp/qemu-go-guide.md using docmgr import file --file ... --ticket ... and read it, and then implement what it suggest.

Create a detailed implementation guide.

Create a detailed analysis / design / implementation guide that is very detailed for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file
  references.
  It should be very clear and detailed. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Create the ticketed workspace, import the provided guide into it, build the QEMU-backed Go `/init` proof of concept the guide recommends, and write intern-friendly design and implementation documentation before uploading the result to reMarkable.

**Inferred user intent:** Turn an empty repository into both a working educational proof of concept and a well-documented ticket deliverable that can be reviewed and handed to someone junior.

**Commit (code):** N/A — uncommitted workspace

### What I did

- Ran `docmgr init --seed-vocabulary`.
- Created ticket `QEMU-GO-INIT-001`.
- Added the primary design doc and the diary doc.
- Imported `/tmp/qemu-go-guide.md` with `docmgr import file --ticket QEMU-GO-INIT-001 --file /tmp/qemu-go-guide.md`.
- Read the imported guide and extracted its core implementation sequence: static Go `/init`, initramfs, QEMU boot, host port forwarding.

### Why

- A ticket workspace was required for storing the design deliverables and for later reMarkable upload.
- Importing the source document made the evidence chain explicit.
- The imported guide gave the first implementation target and helped avoid scope drift.

### What worked

- `docmgr init --seed-vocabulary` created `ttmp/` and `.ttmp.yaml`.
- `docmgr ticket create-ticket --ticket QEMU-GO-INIT-001 --title "Single-binary Go init that serves a webpage in QEMU" --topics go,qemu,linux,initramfs,web` succeeded.
- `docmgr import file --ticket QEMU-GO-INIT-001 --file /tmp/qemu-go-guide.md` copied the source into `sources/local/qemu-go-guide.md`.

### What didn't work

- `docmgr status --summary-only` initially failed because the workspace did not exist yet.

```text
Error: root directory does not exist: /home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp
```

### What I learned

- This repo had no pre-existing scaffolding, so the ticket setup was not paperwork; it was the first real artifact creation step.
- The imported guide's shell-based initramfs assembly would work, but it would leave unnecessary privilege and tooling friction in the repo.

### What was tricky to build

- The imported guide assumes a developer can create `/dev/console` and `/dev/null` in a staging tree. In a normal non-root workflow that means `mknod` becomes the first friction point. I decided early to absorb that complexity into a Go `newc` writer instead of shipping a repo workflow that depends on `sudo`.

### What warrants a second pair of eyes

- The ticket organization is straightforward, but the decision to replace the shell archive flow with a custom writer is a meaningful design choice and deserved explicit documentation.

### What should be done in the future

- Add a follow-up note once a readable kernel image or custom kernel path is available for fully automated guest boot validation.

### Code review instructions

- Start with `ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/sources/local/qemu-go-guide.md`.
- Confirm the ticket contains the imported source plus the new design, implementation, and diary documents.

### Technical details

```bash
docmgr init --seed-vocabulary
docmgr ticket create-ticket --ticket QEMU-GO-INIT-001 --title "Single-binary Go init that serves a webpage in QEMU" --topics go,qemu,linux,initramfs,web
docmgr doc add --ticket QEMU-GO-INIT-001 --doc-type design-doc --title "Single-binary Go init architecture and implementation guide"
docmgr doc add --ticket QEMU-GO-INIT-001 --doc-type reference --title "Diary"
docmgr import file --ticket QEMU-GO-INIT-001 --file /tmp/qemu-go-guide.md
```

## Step 2: Implement the Go init, rootless initramfs builder, and local automation

With the ticket in place, the next step was to turn the imported guide into an actual repository implementation. Because the repo was empty, I created the Go module, the guest entrypoint, the embedded web UI, the initramfs writer, and the host-side build automation together.

The implementation stayed deliberately small, but I avoided a fake demo. The guest really serves a webpage, the initramfs is really generated from source, and the smoke script really attempts to boot QEMU and hit the guest over a forwarded port.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Build the working proof of concept the imported guide outlined and make the repo runnable enough to support the documentation.

**Inferred user intent:** End up with real code, not just a speculative design.

**Commit (code):** N/A — uncommitted workspace

### What I did

- Added `go.mod`.
- Added `cmd/init/main.go`.
- Added `internal/boot/boot.go` for mounts, signal handling, and request logging.
- Added `internal/webui/site.go` and `internal/webui/static/index.html`.
- Added `cmd/mkinitramfs/main.go`.
- Added `internal/initramfs/writer.go` and `internal/initramfs/writer_test.go`.
- Added `Makefile`, `README.md`, and `scripts/qemu-smoke.sh`.
- Ran `gofmt -w ...`.
- Ran `make test`.
- Ran `make initramfs`.
- Ran `make smoke QEMU_HOST_PORT=18080`.

### Why

- The Go `/init` binary is the heart of the demo.
- The rootless initramfs builder converts the imported guide from a shell sketch into a reproducible repository workflow.
- The smoke script is the fastest way to verify the host-to-guest HTTP path when a readable kernel image exists.

### What worked

- `make test` passed after fixing the archive test parser.
- `make initramfs` successfully produced `build/init` and `build/initramfs.cpio.gz`.
- `file build/init build/initramfs.cpio.gz` confirmed the init binary is statically linked and the archive is gzip-compressed.

### What didn't work

- The first `make test` run failed because the test parser used the wrong `newc` header offsets for `rdevmajor`, `rdevminor`, and `namesize`.

```text
panic: runtime error: slice bounds out of range [:-1]
...
/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/initramfs/writer_test.go:107
```

- The first QEMU smoke attempt failed because QEMU could not read the host kernel image:

```text
qemu: could not open kernel file '/boot/vmlinuz-6.8.0-90-generic': Permission denied
```

- After tightening kernel detection, the next smoke attempt failed earlier and more clearly:

```text
KERNEL_IMAGE is not set and no readable /boot/vmlinuz-* image was found. Set KERNEL_IMAGE to a readable bzImage/vmlinuz path.
```

### What I learned

- The initramfs writer itself was correct on the first pass; the broken piece was the test parser used to inspect the `newc` fields.
- The local host environment matters as much as the repo code for QEMU boot validation. On this machine, `/boot/vmlinuz-*` exists but is root-only, so automated boot is blocked until a readable kernel image is provided.

### What was tricky to build

- `newc` is simple in concept but strict in detail. The header field ordering, NUL-terminated names, and four-byte padding all have to match expectations exactly. The test failure was useful because it forced me to verify the field offsets rather than assuming the parser was correct.
- The smoke flow had to distinguish between "guest failed to boot" and "host kernel path is unreadable." Surfacing the kernel readability failure before launch makes later debugging much easier for the next engineer.

### What warrants a second pair of eyes

- `internal/initramfs/writer.go`, because archive-format code is easy to get subtly wrong even when unit tests pass.
- `internal/boot/boot.go`, because PID 1 behavior is always worth reviewing carefully.

### What should be done in the future

- Re-run `make run` and `make smoke` with a readable kernel image and capture that verification result in the ticket.
- Decide whether the next iteration should build a custom kernel or embed the initramfs into one.

### Code review instructions

- Start with `cmd/init/main.go`, then `internal/boot/boot.go`, then `internal/webui/site.go`.
- Review `cmd/mkinitramfs/main.go` and `internal/initramfs/writer.go` together as one build pipeline.
- Validate with:

```bash
make test
make initramfs
KERNEL_IMAGE=/path/to/readable/bzImage make run
```

### Technical details

```bash
gofmt -w cmd/init/main.go cmd/mkinitramfs/main.go internal/boot/boot.go internal/initramfs/writer.go internal/initramfs/writer_test.go internal/webui/site.go
make test
make initramfs
file build/init build/initramfs.cpio.gz
make smoke QEMU_HOST_PORT=18080
```

## Step 3: Write the design and implementation docs and prepare delivery

The last engineering step was to turn the code and validation output into intern-friendly documentation. The user explicitly asked for prose, bullet points, pseudocode, diagrams, API references, and file references, so the ticket deliverables were written as onboarding documents, not just terse change logs.

This step also closed the loop between implementation and explanation. Every major claim in the design doc points back to a concrete source file, and the implementation guide gives a straight-line runbook a new engineer can follow without rediscovering the build sequence.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Package the implementation into durable ticket documentation and prepare it for delivery.

**Inferred user intent:** Hand off something another engineer can actually learn from and continue.

**Commit (code):** N/A — uncommitted workspace

### What I did

- Wrote the primary design document.
- Added a dedicated implementation guide reference doc.
- Recorded this diary.
- Updated the ticket task list and changelog.
- Ran `docmgr doctor --ticket QEMU-GO-INIT-001 --stale-after 30`.
- Uploaded the bundle to `/ai/2026/03/08/QEMU-GO-INIT-001` with `remarquee upload bundle`.

### Why

- The code alone is not sufficient for the user request.
- The target audience is explicitly a new intern, which changes the required detail level.

### What worked

- The ticket now contains a primary design doc, an implementation guide, the imported source, and this diary.
- `docmgr doctor --ticket QEMU-GO-INIT-001 --stale-after 30` returned `All checks passed`.
- `remarquee upload bundle ... --name "QEMU-GO-INIT-001 bundle" --remote-dir "/ai/2026/03/08/QEMU-GO-INIT-001"` succeeded.
- `remarquee cloud ls /ai/2026/03/08/QEMU-GO-INIT-001 --long --non-interactive` showed `[f]	QEMU-GO-INIT-001 bundle`.

### What didn't work

- N/A

### What I learned

- The most useful onboarding docs explain both the target architecture and the operational path to reproduce it.

### What was tricky to build

- Balancing depth and navigation was the main challenge. The design doc needed to be detailed without turning into a wall of unstructured notes, so I separated architecture and runbook concerns into two documents.

### What warrants a second pair of eyes

- The completeness of the intern explanation: a reviewer who is newer to QEMU or Linux boot than I am would be the best test of whether the docs are truly accessible.

### What should be done in the future

- Provide a readable kernel image and rerun the end-to-end QEMU smoke boot on this host.

### Code review instructions

- Read `design-doc/01-single-binary-go-init-architecture-and-implementation-guide.md`.
- Follow with `reference/02-implementation-guide.md`.
- Use this diary only for chronology and validation evidence.

### Technical details

- Design doc focus: architecture, tradeoffs, pseudocode, risks.
- Implementation guide focus: commands, file map, troubleshooting, extension ideas.
- Delivery verification:

```bash
docmgr doctor --ticket QEMU-GO-INIT-001 --stale-after 30
remarquee status
remarquee cloud account --non-interactive
remarquee upload bundle ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/index.md \
  ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/design-doc/01-single-binary-go-init-architecture-and-implementation-guide.md \
  ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/reference/02-implementation-guide.md \
  ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/reference/01-diary.md \
  ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/tasks.md \
  ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/changelog.md \
  --name "QEMU-GO-INIT-001 bundle" \
  --remote-dir "/ai/2026/03/08/QEMU-GO-INIT-001" \
  --toc-depth 2
remarquee cloud ls /ai/2026/03/08/QEMU-GO-INIT-001 --long --non-interactive
```
