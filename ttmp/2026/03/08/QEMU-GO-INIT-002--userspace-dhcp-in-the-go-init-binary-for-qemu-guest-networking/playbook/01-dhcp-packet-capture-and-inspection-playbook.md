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
LastUpdated: 2026-03-08T16:23:00-04:00
WhatFor: ""
WhenToUse: ""
---


# DHCP packet capture and inspection playbook

## Current Status

This workflow is now known-good in the repo.

Resolved on 2026-03-08:

- the guest successfully emits DHCP Discover and Request packets,
- QEMU replies with Offer and ACK,
- the Go init applies `10.0.2.15/24` with gateway `10.0.2.2`,
- the host reaches:
  - `http://127.0.0.1:18080/healthz`
  - `http://127.0.0.1:18080/`
  - `http://127.0.0.1:18080/api/status`

Root cause of the earlier silent stall:

- the previous code called the library helper `nclient4.Request(...)`
- that helper eventually called `dhcpv4.NewDiscovery(...)`
- `dhcpv4.NewDiscovery(...)` internally called `dhcpv4.New()`
- `dhcpv4.New()` generated a random transaction ID before modifiers were applied
- in this tiny initramfs, early-boot entropy was unreliable, so packet construction could stall before any DHCP frame was sent

The fix was to drive the Discover/Offer/Request/Ack handshake explicitly with a deterministic transaction ID and the library's raw interface socket.

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

### 6. Inspect the runtime watchdog logs around the DHCP call

The current guest runtime emits a DHCP wait watchdog every 5 seconds while the request is blocked.

```bash
grep -Ei 'requesting DHCP lease|still waiting for DHCP|DHCP wait|fallback|configured' build/qemu-smoke.log
```

This tells you whether the Go process is still alive during DHCP, whether the request path is progressing, and whether the QEMU static fallback path activated.

### 7. Use a bounded outer timeout when you are reproducing a suspected stall

The current code path succeeds without this, but a bounded outer timeout is still useful when you are testing a risky networking change:

```bash
timeout 75s env \
  QEMU_PCAP=/tmp/qemu-net.pcap \
  KERNEL_IMAGE=/tmp/qemu-vmlinuz \
  QEMU_HOST_PORT=18080 \
  make smoke
```

The old failure signature looked like this:

```text
networking: requesting DHCP lease on eth0 xid=... deadline=45s
networking: still waiting for DHCP on eth0 xid=... elapsed=5.008s
...
networking: DHCP wait on eth0 xid=... ended via context after 44.997s err=context deadline exceeded
qemu-system-x86_64: terminating on signal 15 from pid ... (timeout)
```

This combination meant that:

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

Known-good example observed on 2026-03-08:

```text
2026/03/08 20:20:11.859805 networking: dhcp-raw(eth0) write start bytes=300 addr=255.255.255.255:67
2026/03/08 20:20:11.925482 dhcp: sent message DHCPv4 Message ... DHCP Message Type: DISCOVER
2026/03/08 20:20:11.934069 dhcp: received message DHCPv4 Message ... DHCP Message Type: OFFER
2026/03/08 20:20:11.950242 dhcp: sent message DHCPv4 Message ... DHCP Message Type: REQUEST
2026/03/08 20:20:11.971370 dhcp: received message DHCPv4 Message ... DHCP Message Type: ACK
2026/03/08 20:20:12.345060 networking: configured eth0 with 10.0.2.15/24 gateway=10.0.2.2 dns=10.0.2.3
2026/03/08 20:20:12.463390 go init ready on :8080
2026/03/08 20:20:13.282745 GET /healthz from 10.0.2.2:53694
```

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
- if there are also no `dhcp-raw(... ) write start` lines in `build/qemu-smoke.log`, the stall is probably above the socket layer

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

The most useful lines are currently:

- `networking: discovered links => ...`
- `networking: opened raw DHCP socket on eth0 local=...`
- `networking: dhcp-raw(eth0) write start ...`
- `dhcp: sent message DHCPv4 Message`
- `dhcp: received message DHCPv4 Message`
- `networking: configured eth0 with ...`

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

- The temporary `dhcp-raw(... ) read error: ... use of closed file` line appears when the client closes its packet conn after a successful lease. It is expected with the current debug wrapper and should not be confused with a network failure.
- If packet capture becomes part of the standard smoke workflow, add a debug mode to `scripts/qemu-smoke.sh` that writes `/tmp/qemu-net.pcap` automatically on failure.
- If the final design keeps the QEMU user-net fallback, this playbook should explicitly show how to distinguish the fallback path from a real DHCP success path in `/api/status`.
