---
Title: Diary
Ticket: QEMU-GO-INIT-003
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
RelatedFiles:
    - Path: Makefile
      Note: Ticket step 1 analysis of QEMU launch surfaces
    - Path: scripts/qemu-smoke.sh
      Note: Ticket step 1 analysis of smoke workflow surfaces
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T16:34:44.588499661-04:00
WhatFor: ""
WhenToUse: ""
---


# Diary

## Goal

Capture the implementation steps for explicit early-boot entropy support in the QEMU Go init runtime, including each code slice, validation command, commit, and ticket-bookkeeping update.

## Step 1: Create the entropy-support ticket and scope the first implementation slices

This step created the dedicated ticket and turned the high-level entropy request into an implementable sequence. The repo already had the key preconditions for this work: a working QEMU smoke path, a Go PID 1 runtime, and a previous postmortem that proved early-boot randomness is an operational dependency rather than an incidental detail.

The immediate scope was kept deliberately narrow. This ticket will add QEMU `virtio-rng` support, guest entropy diagnostics, and UI/API visibility. It will not attempt to solve persistent seed storage or general-purpose secret lifecycle management in the same change set.

### Prompt Context

**User prompt (verbatim):** "alright, add a ticket to do that with docmgr, and add an implementation plan and tasks, and then work them off one by one, comitting as you go and keeping a detailed diary."

**Assistant interpretation:** Create a new entropy-focused ticket, document the implementation plan and task list in detail, then execute the work incrementally with commits and a detailed diary.

**Inferred user intent:** Turn the entropy recommendations into a real implementation effort with disciplined ticketing, code changes, and engineering documentation.

**Commit (code):** pending

### What I did

- Created ticket `QEMU-GO-INIT-003`.
- Added:
  - a design doc,
  - an implementation guide,
  - a diary.
- Read the current implementation surfaces in:
  - [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
  - [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
  - [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
  - [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
- Wrote the initial task breakdown and architecture notes.

### Why

- The repo already had enough evidence to start implementing entropy support directly.
- Ticket-first execution keeps the code, docs, and commit history aligned.

### What worked

- The ticket scaffold now reflects the real code surfaces and a realistic implementation sequence.

### What didn't work

- `docmgr doc add --ticket QEMU-GO-INIT-003 --doc-type design-doc ...` briefly raced ticket creation and returned:

```text
Error: failed to find ticket directory: ticket not found: QEMU-GO-INIT-003
```

- Re-running the command after the workspace existed fixed it.

### What I learned

- The work naturally breaks into three implementation slices:
  - QEMU `virtio-rng` launch plumbing,
  - guest-side entropy diagnostics,
  - UI/API exposure and validation.

### What was tricky to build

- The main challenge here was scope control. “Handle actual entropy generation” can easily expand into seed persistence, key management, health gating, and kernel policy. This ticket needs a first slice that materially improves the environment without pretending to solve all entropy concerns at once.

### What warrants a second pair of eyes

- The line between “support and diagnostics” and “full entropy lifecycle management” is partly a product decision. If the project intends to generate real secrets in early boot immediately, a follow-up ticket for seed persistence may need to happen soon.

### What should be done in the future

- Land the first code slice: QEMU `virtio-rng` support.
- Then add guest diagnostics and UI wiring.

### Code review instructions

- Start with:
  - [tasks.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/tasks.md)
  - [01-early-boot-entropy-support-architecture-and-implementation-guide-for-the-qemu-go-init-runtime.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/design-doc/01-early-boot-entropy-support-architecture-and-implementation-guide-for-the-qemu-go-init-runtime.md)
- Then confirm the mapped implementation surfaces in the repo files listed above.

### Technical details

Commands used:

```bash
docmgr ticket create-ticket --ticket QEMU-GO-INIT-003 --title "Add explicit early-boot entropy support and diagnostics for the QEMU Go init runtime" --topics go,qemu,linux,initramfs,web,networking
docmgr doc add --ticket QEMU-GO-INIT-003 --doc-type design-doc --title "Early-boot entropy support architecture and implementation guide for the QEMU Go init runtime"
docmgr doc add --ticket QEMU-GO-INIT-003 --doc-type reference --title "Diary"
docmgr doc add --ticket QEMU-GO-INIT-003 --doc-type reference --title "Implementation guide"
```

## Step 2: Add `virtio-rng` support to the repo's QEMU boot paths

This step landed the environment change first. The goal was to make the standard repo boot path expose a virtual RNG device before changing the guest runtime itself, so validation could answer a simpler question: does the guest still boot cleanly when the QEMU environment is enriched with entropy support?

The changes stayed intentionally small and local to the QEMU launch surfaces. That made it possible to validate the environment update independently and commit it cleanly as its own slice.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Work through the entropy ticket in small, reviewable slices and commit each completed slice separately.

**Inferred user intent:** Make incremental progress that is easy to validate and audit.

**Commit (code):** a8651a6b9d00851c09ea616a3876773ada9a501c — "Add virtio-rng to QEMU boot paths"

### What I did

- Updated [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile) to support:
  - `QEMU_ENABLE_VIRTIO_RNG`
  - `QEMU_RNG_OBJECT`
  - `QEMU_RNG_DEVICE`
- Updated [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh) to append:
  - `-object rng-random,id=rng0,filename=/dev/urandom`
  - `-device virtio-rng-pci,rng=rng0`
  when enabled.
- Updated [README.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/README.md) to document the new QEMU entropy environment knobs.
- Verified that `qemu-system-x86_64 -device help` includes `virtio-rng-pci`.
- Re-ran the smoke boot with `/tmp/qemu-vmlinuz`.

### Why

- The guest cannot consume an entropy device that the virtual hardware never exposes.
- Landing the QEMU slice separately keeps the history readable and the validation surface small.

### What worked

- Local QEMU supports `virtio-rng-pci`.
- The smoke boot still completed successfully after the launch-path change.
- `build/qemu-smoke.log` now records `virtio_rng=enabled` in the first line.

### What didn't work

- N/A

### What I learned

- The repo's QEMU path was a clean seam for this work. The Makefile and smoke script already centralized the relevant arguments.

### What was tricky to build

- The main sharp edge was keeping the QEMU flags configurable while still enabling them by default in the repo run path.

### What warrants a second pair of eyes

- The default decision to enable `virtio-rng` in repo run paths. It is the right default for this ticket, but it is still a behavior change for anyone relying on strict parity with the older launch args.

### What should be done in the future

- Add guest-side diagnostics so the running system can confirm what the launch environment actually produced.

### Code review instructions

- Review:
  - [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
  - [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
- Validate with:

```bash
qemu-system-x86_64 -device help | rg 'virtio-rng-pci'
timeout 45s env KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 make smoke
```

### Technical details

The QEMU log prefix after this step:

```text
qemu-smoke: kernel=/tmp/qemu-vmlinuz host_port=18080 guest_port=8080 model=virtio-net-pci pcap=disabled virtio_rng=enabled
```

## Step 3: Add guest entropy diagnostics and uncover the kernel-module blocker

This step made entropy state visible from inside the guest. The new code probes kernel and sysfs surfaces, exposes them through `/api/status`, and renders them in the embedded webpage so the operator can see whether the guest actually has an active RNG backend instead of just assuming the QEMU launch args were enough.

That visibility immediately paid off. The guest now clearly reports that `/dev/hwrng` exists and `entropy_avail` is readable, but `rng_current` is still `none`. That led to the next important finding: the current host kernel config used for the QEMU boot has `CONFIG_HW_RANDOM_VIRTIO=m`, so the driver is not built in and is unavailable in this module-less initramfs.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Continue the entropy ticket by wiring guest-side diagnostics, validating them, and recording what the guest actually sees.

**Inferred user intent:** Move from environment assumptions to host-verifiable runtime truth.

**Commit (code):** a7af70d9cf23de6aa50175f9d9b1cbe134bbe546 — "Add guest entropy diagnostics"

### What I did

- Added [internal/entropy/entropy.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy.go).
- Added [internal/entropy/entropy_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy_test.go).
- Updated [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go) to probe entropy after networking setup.
- Updated [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go) to include entropy in `/api/status`.
- Updated [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html) to render an Entropy panel.
- Re-ran:

```bash
make test
timeout 60s env KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 make smoke
grep -n 'CONFIG_HW_RANDOM_VIRTIO\|CONFIG_HW_RANDOM' /boot/config-$(uname -r)
grep -Ei 'rng|virtio' build/qemu-smoke.log
```

### Why

- If the runtime cannot tell the operator what it actually sees, the QEMU-side entropy change is not operationally trustworthy.
- The status page is the natural place for this visibility because the repo is already built around host-visible inspection of a tiny guest.

### What worked

- The new entropy package passed unit tests.
- `make test` passed.
- The smoke boot passed.
- `/api/status` now returns:

```json
"entropy": {
  "entropyAvail": 23,
  "entropyAvailKnown": true,
  "hwrngDevice": true,
  "rngCurrent": "none",
  "virtioRngVisible": false
}
```

### What didn't work

- The guest did not activate a visible `virtio-rng` backend even though the QEMU launch path enabled the device.

### What I learned

- The current kernel image used for the guest boot is the real blocker now, not the repo's QEMU or userspace plumbing.
- `/boot/config-$(uname -r)` shows:

```text
CONFIG_HW_RANDOM=y
CONFIG_HW_RANDOM_VIRTIO=m
```

- Since the initramfs does not ship modules, the guest cannot activate `virtio_rng` in the current boot environment.

### What was tricky to build

- The subtle part was distinguishing “QEMU is not exposing the device” from “the kernel sees the device but cannot activate the driver.” The entropy probe made that distinction visible.

### What warrants a second pair of eyes

- Whether the next step should be a custom kernel with `CONFIG_HW_RANDOM_VIRTIO=y` or an initramfs module-loading path for `virtio_rng.ko`.

### What should be done in the future

- Decide and implement the kernel-side follow-up needed for actual virtio-rng activation.
- Optionally surface a warning in the UI when `/dev/hwrng` exists but no active RNG backend is selected.

### Code review instructions

- Review:
  - [internal/entropy/entropy.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy.go)
  - [internal/entropy/entropy_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy_test.go)
  - [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
  - [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
  - [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html)
- Validate with:

```bash
make test
timeout 60s env KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 make smoke
```

### Technical details

Observed runtime log:

```text
2026/03/08 20:43:49.055288 entropy: entropy_avail=23 hwrng=true rng_current="none" rng_available=<none> virtio_rng_visible=false warnings=<none>
```
