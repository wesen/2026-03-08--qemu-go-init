---
Title: DHCP packet capture and inspection playbook
Ticket: QEMU-GO-INIT-002
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - web
    - networking
DocType: playbook
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Step-by-step packet capture and inspection workflow for debugging DHCP inside the QEMU guest.
LastUpdated: 2026-03-08T15:59:00-04:00
WhatFor: ""
WhenToUse: ""
---

# DHCP packet capture and inspection playbook

## Purpose

This playbook captures DHCP and early guest network traffic at the QEMU NIC boundary, then inspects it with `tshark` or `tcpdump` so the engineer can determine exactly where the userspace DHCP flow is failing.

## Environment Assumptions

- Repository root: `/home/manuel/code/wesen/2026-03-08--qemu-go-init`
- Readable kernel image: `/tmp/qemu-vmlinuz`
- Built initramfs: `build/initramfs.cpio.gz`
- Tools available on host:
  - `qemu-system-x86_64`
  - `tshark`
  - `tcpdump`
- QEMU user networking is used, not TAP/bridge mode.

## Background

QEMU `-nic user` uses the internal `user` backend. That backend provides NAT, host forwarding, and a DHCP service for the guest network. Because the DHCP server is internal to QEMU user networking, a normal host-interface capture is not the simplest debugging tool. The most direct capture point is QEMU's own `filter-dump` object attached to the guest netdev.

## Commands

### 1. Rebuild the guest artifact

```bash
cd /home/manuel/code/wesen/2026-03-08--qemu-go-init
make initramfs
```

### 2. Start QEMU with packet capture enabled

This command writes a pcap file at `/tmp/qemu-net.pcap` and keeps the serial console in the terminal.

```bash
qemu-system-x86_64 \
  -m 512 \
  -nographic \
  -kernel /tmp/qemu-vmlinuz \
  -initrd build/initramfs.cpio.gz \
  -append 'console=ttyS0 rdinit=/init' \
  -netdev user,id=n1,hostfwd=tcp::18080-:8080 \
  -device virtio-net-pci,netdev=n1 \
  -object filter-dump,id=f1,netdev=n1,file=/tmp/qemu-net.pcap
```

### 3. In another terminal, inspect just DHCP traffic

```bash
tshark -r /tmp/qemu-net.pcap -Y 'bootp || dhcp'
```

Alternative:

```bash
tcpdump -nn -r /tmp/qemu-net.pcap port 67 or port 68
```

### 4. Inspect the full guest network conversation

```bash
tshark -r /tmp/qemu-net.pcap
```

### 5. Correlate traffic with guest logs

```bash
grep -Ei 'networking:|dhcp:|fatal:' build/qemu-smoke.log
```

If you are not using the smoke script, copy the QEMU serial output into a file or tmux pane capture and compare timestamps manually.

## Expected Good Flow

You want to see the four DHCPv4 DORA packets:

1. `DHCP Discover` from guest to broadcast
2. `DHCP Offer` from QEMU DHCP service
3. `DHCP Request` from guest
4. `DHCP ACK` from QEMU DHCP service

After that, the guest should:

- apply `10.0.2.x` guest addressing or the leased DHCP address,
- install a default route,
- start the HTTP server,
- respond on `http://127.0.0.1:18080/healthz`

## Failure Interpretation

### Case 1: No DHCP packets at all

Likely causes:

- DHCP client never sends
- socket setup failed before send
- request construction blocked before packet emission

Inspect:

- QEMU serial log
- `networking:` logs in `/init`
- any `fatal:` line before HTTP startup

### Case 2: Discover only, no Offer

Likely causes:

- packet never reaches the QEMU DHCP service
- wrong guest NIC model or broken packet path
- socket/broadcast behavior mismatch

Inspect:

- QEMU netdev/device flags
- NIC model (`virtio-net-pci` vs another model)
- whether the client is broadcasting correctly

### Case 3: Offer arrives, but no Request

Likely causes:

- client-side offer parsing bug
- request-construction bug
- transaction ID mismatch

Inspect:

- DHCP logs around offer handling
- transaction ID values in logs and in the pcap

### Case 4: Full DORA exchange completes, but host HTTP still fails

Likely causes:

- address/route application bug
- resolver or route state not applied correctly
- HTTP server not started after network completion

Inspect:

- `/api/status`
- `networking:` configured log line
- netlink address and route application code

## Extra Debug Variants

### Run with more Go-side logging

```bash
KERNEL_IMAGE=/tmp/qemu-vmlinuz \
QEMU_HOST_PORT=18080 \
make smoke
```

Then inspect:

```bash
tail -n 120 build/qemu-smoke.log
```

### Keep the VM alive in tmux and capture repeatedly

```bash
tmux new-session -d -s qemu-dhcp-debug \
  'cd /home/manuel/code/wesen/2026-03-08--qemu-go-init && \
   qemu-system-x86_64 -m 512 -nographic \
   -kernel /tmp/qemu-vmlinuz \
   -initrd build/initramfs.cpio.gz \
   -append "console=ttyS0 rdinit=/init" \
   -netdev user,id=n1,hostfwd=tcp::18080-:8080 \
   -device virtio-net-pci,netdev=n1 \
   -object filter-dump,id=f1,netdev=n1,file=/tmp/qemu-net.pcap'
```

Then:

```bash
tmux capture-pane -pt qemu-dhcp-debug:0.0
```

## Exit Criteria

This playbook is successful when you can answer all of these with evidence:

- Did the guest emit DHCP packets?
- Did QEMU respond with an Offer and ACK?
- Did `/init` log successful network configuration?
- Did the host receive a successful response from `http://127.0.0.1:18080/healthz`?

## Notes For Future Refinement

- If packet capture becomes part of the standard smoke workflow, add a debug mode to `scripts/qemu-smoke.sh` that writes `/tmp/qemu-net.pcap` automatically on failure.
- If the final design keeps the QEMU user-net fallback, this playbook should explicitly show how to distinguish the fallback path from a real DHCP success path in `/api/status`.
