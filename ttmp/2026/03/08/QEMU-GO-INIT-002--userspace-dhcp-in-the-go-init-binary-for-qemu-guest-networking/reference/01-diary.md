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
RelatedFiles:
    - Path: internal/networking/network.go
      Note: DHCP watchdog
    - Path: scripts/qemu-smoke.sh
      Note: Bounded QEMU smoke workflow and packet capture reproduction
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

## Step 2: Land the userspace networking implementation and validate the first boot path

After the ticket scaffold was in place, the next step was to make the Go init binary own guest network setup. That required pulling in a DHCP library, a netlink library for address and route programming, and a new runtime package to keep the networking logic separate from the existing boot and UI code.

The first implementation pass intentionally kept the system design straightforward. `/init` now mounts the early filesystems, runs userspace network configuration, and only starts the HTTP server once the network step completes. That sequencing makes failures obvious: if the host-forwarded page is unreachable, the QEMU console tells us whether the guest died in mount setup, interface setup, DHCP, address application, or HTTP startup.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Implement the self-contained userspace DHCP path and integrate it into the existing init runtime.

**Inferred user intent:** Replace the missing kernel `ip=dhcp` path with Go-owned guest network setup.

**Commit (code):** pending

### What I did

- Added `github.com/insomniacslk/dhcp` and `github.com/vishvananda/netlink`.
- Created `internal/networking/network.go` and `internal/networking/network_test.go`.
- Wired networking into `cmd/init/main.go`.
- Extended `internal/webui/site.go` and `internal/webui/static/index.html` to expose network state.
- Removed `ip=dhcp` from the default QEMU append string in `Makefile` and `scripts/qemu-smoke.sh`.
- Updated `README.md` with the new userspace-DHCP environment knobs.
- Ran `go mod tidy`, `make test`, and `make initramfs`.

### Why

- The DHCP client handles lease acquisition.
- Netlink configures the Linux interface, address, and route inside the guest.
- The web UI needed to show network status so debugging would not depend only on serial-console logs.

### What worked

- `go mod tidy` resolved the DHCP and netlink dependency graph cleanly.
- `make test` passed with the new networking package.
- `make initramfs` rebuilt a bootable archive containing the updated `/init`.

### What didn't work

- The first `go get` sequence raced on `go.mod` updates and had to be rerun serially:

```text
go: updating go.mod: existing contents have changed since last read
```

- The initial test/build pass failed until transitive DHCP dependencies were written into `go.sum`:

```text
missing go.sum entry for module providing package github.com/u-root/uio/uio
missing go.sum entry for module providing package github.com/mdlayher/packet
```

### What I learned

- The implementation surface is not large once the right libraries are chosen. Most of the complexity is operational: interface selection, lease decoding, route application, and failure visibility.

### What was tricky to build

- The `nclient4` package is usable, but it assumes certain early-boot conditions that are fragile in a tiny guest. That meant the first compile success was not the interesting milestone; the first real QEMU boot was.

### What warrants a second pair of eyes

- `internal/networking/network.go`, because it crosses package boundaries, Linux syscalls, and runtime sequencing.

### What should be done in the future

- Split clean success-path behavior from QEMU-specific fallbacks more explicitly once the DHCP client path is fully understood.

### Code review instructions

- Start with `internal/networking/network.go`.
- Then check how `cmd/init/main.go` sequences networking before `webui.NewHandler`.
- Finally verify the UI/API changes in `internal/webui/site.go` and `internal/webui/static/index.html`.

### Technical details

```bash
go get github.com/insomniacslk/dhcp@latest
go get github.com/vishvananda/netlink@v1.3.1
go mod tidy
make test
make initramfs
```

## Step 3: Debug the DHCP client path inside QEMU and refine the fallback strategy

The first end-to-end boots showed the guest getting farther than before, but not all the way through to a reachable host HTTP endpoint. The console logs confirmed that `/init` now brought `eth0` up and entered the userspace networking step, which meant the project had moved from a kernel-config problem into a real userspace client-debugging problem.

That debugging work exposed two distinct failure modes. First, the DHCP library tried to generate a transaction ID from kernel entropy and blocked in early boot. Second, after replacing that with a deterministic per-boot transaction ID, the DHCP request still did not complete, even after switching from the raw packet socket path to a broadcast UDP socket path. At that point, the most pragmatic next move was to preserve the DHCP attempt and diagnostics, but add a QEMU user-net fallback profile so the guest could still become reachable for the rest of the demo.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Continue working the tasks down while keeping the diary detailed and the implementation testable.

**Inferred user intent:** Preserve forward progress even while the DHCP path is being debugged.

**Commit (code):** pending

### What I did

- Booted the updated initramfs under QEMU with `/tmp/qemu-vmlinuz`.
- Read `build/qemu-smoke.log` after each attempt.
- Added deterministic DHCP transaction ID generation to avoid entropy stalls.
- Replaced the first DHCP socket strategy with an alternate broadcast-UDP strategy.
- Added a QEMU user-net fallback profile (`10.0.2.15/24`, gateway `10.0.2.2`, DNS `10.0.2.3`) that activates if DHCP fails.
- Kept the DHCP attempt and error in the result payload for observability.

### Why

- The guest needed a path to become reachable even if early-boot DHCP remained unreliable.
- Preserving the DHCP attempt keeps the system honest and keeps the debugging data visible.

### What worked

- The guest now logs `networking: link eth0 is up`.
- The deterministic transaction ID removed the entropy-related failure mode.
- Unit tests remained green throughout the DHCP and fallback iterations.

### What didn't work

- The first DHCP client path failed before any packet exchange because transaction ID generation blocked on entropy:

```text
fatal: configure networking: request DHCP lease on eth0: unable to receive an offer: unable to create a discovery request: could not get random number: context deadline exceeded
```

- After that fix, the DHCP request still stalled after startup:

```text
networking: requesting DHCP lease on eth0 xid=0x597be1d0
```

and no corresponding offer or ACK appeared before timeout.

### What I learned

- The current guest problem is no longer “kernel can’t do DHCP.” It is specifically “the userspace DHCP exchange is not completing under this boot/runtime combination.”
- Early-boot entropy assumptions matter even in tiny networking demos.

### What was tricky to build

- The hardest part was not writing the Go code. It was separating three similar-looking failure classes:
  - no network driver,
  - no DHCP/IP autoconfig in the kernel,
  - userspace DHCP client stalls for its own reasons.
- Once those were separated, the logs became much more actionable.

### What warrants a second pair of eyes

- The QEMU user-net fallback values and their long-term place in the design. They are pragmatic and likely correct for this demo, but they are intentionally environment-specific.

### What should be done in the future

- Add packet capture and inspection into the normal debugging loop so DHCP failures can be diagnosed from traffic rather than inference.
- Decide whether the final design should keep the QEMU fallback permanently or reserve it for smoke/debug mode only.

### Code review instructions

- Inspect the networking logs in `build/qemu-smoke.log`.
- Review the deterministic XID generation and the fallback logic in `internal/networking/network.go`.
- Re-run `make smoke QEMU_HOST_PORT=18080 KERNEL_IMAGE=/tmp/qemu-vmlinuz` and compare the console output to the diary notes.

### Technical details

```bash
make smoke QEMU_HOST_PORT=18080 KERNEL_IMAGE=/tmp/qemu-vmlinuz
tail -n 120 build/qemu-smoke.log
grep -Ei 'virtio|eth0|dhcp|offer|ack|fatal' build/qemu-smoke.log
```

## Step 4: Increase DHCP logging, validate packet capture, and backfill the debugging playbook

This step turned the DHCP debug loop from inference into evidence. I added runtime logs that show the available interfaces, the pre-DHCP address/route state, the local DHCP socket binding, and a watchdog that emits progress every five seconds while the DHCP request is blocked.

I also ran the new capture workflow under a bounded host-side timeout so the result was reproducible: the guest reaches the DHCP request call, no BOOTP/DHCP packets appear in the QEMU pcap, and the watchdog logs continue until the inner context expires.

### Prompt Context

**User prompt (verbatim):** "keep a diary as you work an dupdate th elaybook too, detailed. Backfill. Then continue"

**Assistant interpretation:** Record the logging/playbook work in detail, backfill the missing debugging narrative, and then keep progressing the DHCP task.

**Inferred user intent:** Make the ticket documentation continuation-safe for a future engineer while preserving forward momentum on the actual debugging work.

**Commit (code):** ecaf7d366ba0dc6612145ab82da5c9fbc61aa375 — "Implement userspace DHCP guest networking"

### What I did

- Added interface inventory logging and pre-DHCP address/route logging in `/init`.
- Added a DHCP wait watchdog that logs every 5 seconds until the request returns or the context expires.
- Logged the local DHCP socket binding (`0.0.0.0:68`) and the total request deadline.
- Re-ran `make test` after the logging changes.
- Ran a bounded smoke reproduction with packet capture:

```bash
timeout 75s env QEMU_PCAP=/tmp/qemu-net.pcap KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 make smoke
```

- Read the resulting guest log and packet capture with:

```bash
tail -n 120 build/qemu-smoke.log
tshark -r /tmp/qemu-net.pcap -Y 'bootp || dhcp'
tshark -r /tmp/qemu-net.pcap | tail -n 20
```

- Updated the packet-capture playbook to prefer the scripted `QEMU_PCAP` flow and to document the exact observed failure signature.

### Why

- The earlier logs showed where the guest entered DHCP, but not whether it remained alive, how long it stayed blocked, or whether packet capture matched the code-path assumptions.
- The playbook needed to reflect the actual workflow that now works in the repo instead of only a hand-written foreground QEMU command.

### What worked

- The guest now emits enough logs to prove the request path is alive:

```text
networking: discovered links => lo(...) | eth0(...)
networking: pre-dhcp addresses on eth0 => <none>
networking: pre-dhcp routes on eth0 => <none>
networking: opened DHCP UDP broadcast socket on eth0 local=0.0.0.0:68
networking: requesting DHCP lease on eth0 xid=0xbac0e533 deadline=45s
networking: still waiting for DHCP on eth0 xid=0xbac0e533 elapsed=5.008s
...
networking: DHCP wait on eth0 xid=0xbac0e533 ended via context after 44.997s err=context deadline exceeded
```

- `QEMU_PCAP=/tmp/qemu-net.pcap make smoke` successfully produced a guest-boundary pcap without needing a separate manual QEMU invocation.
- `make test` stayed green after the logging changes.

### What didn't work

- `tshark -r /tmp/qemu-net.pcap -Y 'bootp || dhcp'` produced no output at all.
- The smoke run still did not become reachable on `127.0.0.1:18080`; the bounded run ended with:

```text
qemu-system-x86_64: terminating on signal 15 from pid 112491 (timeout)
```

- The pcap showed repeated ARP from `10.0.2.2` for `10.0.2.15` plus IPv6 router solicitation/advertisement traffic, but no DHCP DORA exchange.

### What I learned

- The QEMU `filter-dump` capture path is working and is therefore a reliable place to ask, “did any DHCP packet reach the guest NIC boundary?”
- The current problem is more precise now: the Go process enters the DHCP request path and waits, but no DHCP packets are visible at the QEMU boundary during that wait.
- Repeated ARP from QEMU’s user-net side does not imply successful guest IPv4 configuration.

### What was tricky to build

- The subtle part was separating “no DHCP packets are visible” from “the guest never reached DHCP code.” Without the watchdog logs, those two cases look similar from the host.
- The smoke script needed a host-side outer timeout for reliable iteration because the inner DHCP request path can outlive the normal smoke expectations.

### What warrants a second pair of eyes

- Whether the current socket setup is correct for `nclient4` in this initramfs environment, since we now know the call blocks without emitting visible DHCP packets.
- Whether the packet capture point can miss any part of the current request path, though the absence of BOOTP in `filter-dump` strongly suggests the send path itself is the next place to inspect.

### What should be done in the future

- Inspect the DHCP send path more directly, potentially by instrumenting or replacing the library request path for one debug iteration so every send/receive boundary is explicit.
- Consider trying interface-bound socket behavior next if the current broad `0.0.0.0:68` binding remains silent.
- Keep the host-side bounded reproduction command in the debugging loop until the DHCP request is either fixed or replaced.

### Code review instructions

- Start with [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go) and read the `Configure`, `logInterfaceInventory`, `logAppliedState`, and `logDHCPWait` functions.
- Then read [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh) to see how `QEMU_PCAP` is wired into the smoke workflow.
- Validate with:

```bash
make test
timeout 75s env QEMU_PCAP=/tmp/qemu-net.pcap KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 make smoke
tail -n 120 build/qemu-smoke.log
tshark -r /tmp/qemu-net.pcap -Y 'bootp || dhcp'
```

### Technical details

Relevant files:

- [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go)
- [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
- [playbook/01-dhcp-packet-capture-and-inspection-playbook.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/playbook/01-dhcp-packet-capture-and-inspection-playbook.md)

Key observed command outputs:

```text
2026/03/08 20:08:55.699562 networking: requesting DHCP lease on eth0 xid=0xbac0e533 deadline=45s
2026/03/08 20:09:00.719325 networking: still waiting for DHCP on eth0 xid=0xbac0e533 elapsed=5.008s
2026/03/08 20:09:40.712573 networking: DHCP wait on eth0 xid=0xbac0e533 ended via context after 44.997s err=context deadline exceeded
```

```text
$ tshark -r /tmp/qemu-net.pcap -Y 'bootp || dhcp'
# no output
```
