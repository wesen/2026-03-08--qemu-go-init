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
RelatedFiles:
    - Path: internal/networking/network.go
      Note: Runtime log lines referenced by the playbook
    - Path: scripts/qemu-smoke.sh
      Note: QEMU_PCAP-based packet capture entrypoint
ExternalSources: []
Summary: Step-by-step packet capture and inspection workflow for debugging DHCP inside the QEMU guest.
LastUpdated: 2026-03-08T16:08:00-04:00
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

Preferred path: use the repo smoke script, because it also captures the guest serial log in `build/qemu-smoke.log`.

```bash
cd /home/manuel/code/wesen/2026-03-08--qemu-go-init
QEMU_PCAP=/tmp/qemu-net.pcap \
KERNEL_IMAGE=/tmp/qemu-vmlinuz \
QEMU_HOST_PORT=18080 \
make smoke
```

Equivalent direct QEMU command if you want the VM in the foreground:

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

### 6. Inspect the runtime watchdog logs around the blocking DHCP call

The current guest runtime emits a DHCP wait watchdog every 5 seconds while the request is blocked.

```bash
grep -Ei 'requesting DHCP lease|still waiting for DHCP|DHCP wait|fallback|configured' build/qemu-smoke.log
```

This tells you whether the Go process is still alive inside `Request`, whether the request returned via timeout, and whether the QEMU static fallback path activated.

### 7. Use a bounded outer timeout when the smoke script is expected to hang

At the moment, the DHCP client can block long enough that it is useful to cap the whole smoke run from the host side:

```bash
timeout 75s env \
  QEMU_PCAP=/tmp/qemu-net.pcap \
  KERNEL_IMAGE=/tmp/qemu-vmlinuz \
  QEMU_HOST_PORT=18080 \
  make smoke
```

The current expected tail of `build/qemu-smoke.log` is:

```text
networking: requesting DHCP lease on eth0 xid=... deadline=45s
networking: still waiting for DHCP on eth0 xid=... elapsed=5.008s
...
networking: DHCP wait on eth0 xid=... ended via context after 44.997s err=context deadline exceeded
qemu-system-x86_64: terminating on signal 15 from pid ... (timeout)
```

This combination shows that:

- the guest reached the DHCP request path,
- the watchdog logs were active,
- the inner DHCP context expired,
- the outer host timeout cleaned up the still-running VM.

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
- whether the watchdog repeats `still waiting for DHCP` without any matching DHCP packets in `/tmp/qemu-net.pcap`

Observed on 2026-03-08 with `/tmp/qemu-vmlinuz`:

- `tshark -r /tmp/qemu-net.pcap -Y 'bootp || dhcp'` produced no packets
- guest serial logs reached `networking: requesting DHCP lease on eth0 ...`
- QEMU capture still showed IPv6 router solicitation and repeated ARP probes for `10.0.2.15`

Interpretation:

- the NIC path is alive enough for non-DHCP traffic to exist at the QEMU boundary
- the current userspace DHCP request is not producing visible BOOTP/DHCP packets on that boundary
- repeated ARP for `10.0.2.15` from `10.0.2.2` does not prove the guest configured IPv4 successfully

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

### Confirm that QEMU packet capture is actually active

```bash
ls -l /tmp/qemu-net.pcap build/qemu-smoke.log
```

If `build/qemu-smoke.log` shows `pcap=/tmp/qemu-net.pcap` in the first line and the file grows during boot, the capture hook is attached correctly.

### Capture the full packet summary and save it for the ticket diary

```bash
tshark -r /tmp/qemu-net.pcap > /tmp/qemu-net.txt
tshark -r /tmp/qemu-net.pcap -Y 'bootp || dhcp' > /tmp/qemu-dhcp.txt
```

This is useful when you want the diary to record the exact “no DHCP seen” result without embedding the raw binary pcap.

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
- If the DHCP request continues to block without emitting packets, consider instrumenting the request path more aggressively or replacing the client invocation with a lower-level DHCP exchange for one debug build so each send/receive boundary is explicit in the logs.
