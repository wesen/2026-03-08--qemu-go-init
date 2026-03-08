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
    - Path: build/qemu-smoke.log
      Note: step 4 guest boot evidence including exec format error and final success
    - Path: cmd/mkinitramfs/main.go
      Note: step 4 module packaging and decompression work
    - Path: internal/kmod/kmod.go
      Note: step 4 guest module loading and error handling
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

## Step 4: Embed and load the matching `virtio_rng` module from the initramfs

This step solved the remaining kernel-side blocker without recompiling the kernel. The core idea was to treat the initramfs as the missing distribution channel for the `virtio_rng` driver: package the matching module into the archive, load it from the Go PID 1 runtime immediately after mounts come up, and then wait until the kernel reports the backend as active.

The first attempt failed in a way that materially improved the design. Packaging the host's `.ko.zst` file verbatim caused the guest kernel to reject it with `Invalid ELF header magic` and `exec format error`, so the builder had to grow a decompression step that writes an actual ELF `.ko` into the archive. After that change, the smoke path reached the desired final state: `rngCurrent` became `virtio_rng.0`, `entropyAvail` jumped to `256`, and `virtioRngVisible` became `true`.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Finish the ticket by implementing the kernel-side activation path needed for real guest entropy generation and record the exact debugging process.

**Inferred user intent:** Replace the remaining design gap with a concrete, working mechanism that survives code review and teaches the next engineer what actually mattered.

**Commit (code):** 05f75494c45c1cad42798ceb627a247320fbe68c — "Load virtio-rng module from initramfs"

### What I did

- Added [internal/kmod/kmod.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/kmod/kmod.go) and [internal/kmod/kmod_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/kmod/kmod_test.go) to wrap `finit_module`.
- Updated [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go) to:
  - accept a host `virtio-rng` module path,
  - decompress `.ko.zst` inputs,
  - and package the guest file as `/lib/modules/virtio_rng.ko`.
- Added [cmd/mkinitramfs/main_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main_test.go) for archive and decompression coverage.
- Updated [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go) to load the module before networking and wait for visible activation.
- Extended [internal/entropy/entropy.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy.go) with a wait loop, plus tests in [internal/entropy/entropy_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy_test.go).
- Updated [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go) and [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html) to expose the module-loading result.
- Updated [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile), [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh), and [README.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/README.md) for the final build and validation flow.

### Why

- The host kernel already had the right driver as a module, so shipping and loading that module was the shortest path to a real entropy backend.
- Recompiling the kernel would have solved the problem too, but with materially more build and maintenance overhead for this repo.

### What worked

- `make test` passed after the new builder, loader, and probe logic were added.
- `make initramfs` succeeded while embedding the host `virtio_rng` module.
- `timeout 90s env KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 make smoke` passed.
- The final `/api/status` response showed:

```json
"entropy": {
  "entropyAvail": 256,
  "entropyAvailKnown": true,
  "hwrngDevice": true,
  "rngCurrent": "virtio_rng.0",
  "rngAvailable": ["virtio_rng.0"],
  "virtioRngVisible": true
},
"virtioRngModule": {
  "attempted": true,
  "loaded": true,
  "modulePath": "/lib/modules/virtio_rng.ko",
  "step": "loaded"
}
```

### What didn't work

- The first guest module-loading attempt failed when the initramfs carried the compressed `.ko.zst` file directly. The guest log showed:

```text
[    8.584226] Invalid ELF header magic: non-ELF payload
2026/03/08 20:58:04.475266 kmod: module=/lib/modules/virtio_rng.ko.zst attempted=true loaded=false already_loaded=false step=error error="exec format error"
```

- That failure proved the builder needed to decompress the module before packaging instead of relying on the guest kernel to accept the compressed payload in this path.

### What I learned

- `CONFIG_MODULE_DECOMPRESS=y` on the host kernel was not enough to assume that handing compressed bytes to `finit_module` would work in this boot path.
- Making the initramfs carry an actual ELF `.ko` is more explicit and easier to reason about anyway.
- The final success condition should be outcome-based. The smoke script now checks `virtioRngVisible`, not just whether the loader attempted something.

### What was tricky to build

- The sharpest edge was separating three distinct failure modes:
  - QEMU device exposure failure,
  - missing guest driver,
  - compressed-module packaging error.
- The module-loading result plus entropy probe together made those failure modes distinguishable.

### What warrants a second pair of eyes

- The assumption that the host module always matches the kernel image being booted in QEMU. That is true for the current `/tmp/qemu-vmlinuz` flow, but a future kernel-selection feature should make the module path and kernel image pair explicit.

### What should be done in the future

- Add seed persistence once the guest has a writable storage model.
- Decide whether low-entropy or missing-RNG conditions should eventually influence `/healthz`.

### Code review instructions

- Start with:
  - [cmd/mkinitramfs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go)
  - [internal/kmod/kmod.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/kmod/kmod.go)
  - [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go)
- Then check:
  - [internal/entropy/entropy.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/entropy/entropy.go)
  - [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
- Validate with:

```bash
make test
make initramfs
timeout 90s env KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 make smoke
```

### Technical details

Commands used during the debugging loop:

```bash
tail -n 200 build/qemu-smoke.log
file /lib/modules/$(uname -r)/kernel/drivers/char/hw_random/virtio-rng.ko.zst
modinfo -F vermagic virtio_rng
strings /tmp/qemu-vmlinuz | rg '6\.8\.0-101-generic'
go mod tidy
make test
make initramfs
timeout 90s env KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 make smoke
```

## Step 5: Finalize the ticket bundle, fix the PDF rendering issue, and publish the docs

This step closed the ticket loop by validating the final documentation bundle and uploading it to reMarkable. The code work was already done, but the publishing path still surfaced one real issue: pandoc rejected a literal control character copied from the guest kernel log, so the diary needed a small sanitization fix before the bundle could render as PDF.

That was exactly the kind of failure worth preserving. It was not a product bug, but it was a real operational issue in the documentation toolchain, and recording it makes the publication path reproducible for the next engineer instead of looking magically smooth in hindsight.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Finish the ticket completely, including the final documentation validation and publication steps, and keep the diary honest about the friction encountered.

**Inferred user intent:** End with a complete, reviewable engineering artifact set rather than stopping at code completion.

**Commit (code):** pending

### What I did

- Ran:

```bash
docmgr doctor --ticket QEMU-GO-INIT-003 --stale-after 30
remarquee status
remarquee cloud account --non-interactive
remarquee upload bundle --dry-run ...
remarquee upload bundle ...
remarquee cloud ls /ai/2026/03/08/QEMU-GO-INIT-003 --long --non-interactive
```

- Fixed the diary snippet that contained a literal control character copied from the kernel log.
- Re-ran the upload after sanitizing that text.

### Why

- The user asked for a full ticket workflow, not just code.
- A successful upload plus a verified remote listing is the final proof that the document bundle is usable outside the repo.

### What worked

- `docmgr doctor --ticket QEMU-GO-INIT-003 --stale-after 30` passed:

```text
## Doctor Report (1 findings)

### QEMU-GO-INIT-003

- ✅ All checks passed
```

- The final upload succeeded:

```text
OK: uploaded QEMU-GO-INIT-003 bundle.pdf -> /ai/2026/03/08/QEMU-GO-INIT-003
```

- Remote verification succeeded:

```text
[f]	QEMU-GO-INIT-003 bundle
```

### What didn't work

- The first upload attempt failed during PDF generation:

```text
Error: pandoc failed: Error producing PDF.
! Text line contains an invalid character.
l.1966 ...584226] Invalid ELF header magic: != ^^?
```

- The root cause was a literal control character embedded in the copied kernel log line inside the diary.

### What I learned

- Even documentation bundles need an explicit validation loop. Markdown that looks fine in the repo can still fail when routed through PDF tooling.
- Sanitizing copied kernel log output is worthwhile when it contains non-printable bytes.

### What was tricky to build

- The subtle part was recognizing that the upload failure had nothing to do with reMarkable itself; the failure happened earlier in the pandoc-to-PDF stage.

### What warrants a second pair of eyes

- The diary is now safe for PDF rendering, but future ticket docs that paste binary-adjacent log output should be reviewed with the same suspicion.

### What should be done in the future

- Consider a small documentation hygiene rule: sanitize non-printable log output before putting it into long-form docs intended for PDF export.

### Code review instructions

- Review the final publishing artifacts:
  - [tasks.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/tasks.md)
  - [changelog.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/changelog.md)
  - [01-diary.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/reference/01-diary.md)
- Validate by rerunning the same `docmgr doctor` and `remarquee` commands.

### Technical details

The sanitized log line now reads:

```text
[    8.584226] Invalid ELF header magic: non-ELF payload
```
