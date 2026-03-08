---
Title: 'Postmortem: early-boot DHCP entropy stall and recovery'
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
      Note: Final deterministic DHCP handshake and raw socket implementation discussed in the postmortem
    - Path: internal/networking/network_test.go
      Note: Regression tests for deterministic packet builders referenced in the postmortem
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T16:28:08.632523469-04:00
WhatFor: ""
WhenToUse: ""
---


# Postmortem: early-boot DHCP entropy stall and recovery

## Goal

Capture the failure, debugging path, technical root cause, fix, and follow-up actions for the early-boot DHCP stall in the Go `/init` runtime. This document is intended to help future engineers understand both the immediate bug and the broader systems lesson: early-boot networking and early-boot entropy are tightly coupled, and convenience helpers can hide blocking behavior that is unacceptable in a minimal initramfs.

## Context

This ticket added userspace DHCP to a single static Go binary that runs as PID 1 inside a QEMU guest. The binary is responsible for:

- mounting the minimal runtime filesystems,
- discovering the guest NIC,
- obtaining IPv4 configuration,
- applying address, route, and resolver state,
- serving a webpage on port `8080`.

The initial implementation used the `github.com/insomniacslk/dhcp/dhcpv4/nclient4` package and a simple DHCP request flow. The guest booted, the NIC appeared, and the process reached the networking code, but the web server was not reachable from the host. The debugging effort established that:

- QEMU user networking was present and able to provide DHCP,
- the kernel had a built-in `virtio_net` path,
- kernel-side `ip=dhcp` was not available because `CONFIG_IP_PNP` was disabled,
- userspace DHCP was therefore the right architectural direction,
- the first implementation could still stall before any DHCP frame reached the wire.

The final implementation in [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go) now works end to end.

## Quick Reference

### Executive Summary

Incident:

- Guest networking stalled during DHCP in early boot.

Observed symptom:

- Host connections to `127.0.0.1:18080` either hung or never completed.

Immediate misleading hypothesis:

- “The raw socket or broadcast path is broken.”

Actual root cause:

- The upstream DHCP helper path generated a random transaction ID before user-provided modifiers were applied.
- In this tiny initramfs environment, entropy availability was not reliable early in boot.
- Packet construction could therefore block before any DHCP packet was emitted.

Fix:

- Replace the helper-based `Request()` path with an explicit deterministic-xid Discover/Offer/Request/Ack handshake over the raw interface socket.

Validation:

- `make test` passes.
- `make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080` passes.
- Packet capture shows full DHCP DORA.
- `/healthz`, `/`, and `/api/status` all succeed from the host side.

### Timeline

1. The guest booted and discovered `eth0`, but the host could not reach the forwarded HTTP port.
2. Kernel-side DHCP was ruled out because the host kernel ignored `ip=dhcp` due to missing `CONFIG_IP_PNP`.
3. Userspace DHCP was implemented in Go.
4. The first DHCP attempt failed due to blocked random number generation.
5. A deterministic transaction ID was added, but the guest still appeared stuck during DHCP.
6. Packet capture showed no DHCP packets at all, even though the guest reported it was “requesting DHCP lease”.
7. The code was moved down to the library’s raw interface socket path to prove whether Ethernet-level transmission worked.
8. Additional logging showed the process never reached `WriteTo`, which meant the stall was above the raw socket layer.
9. Source inspection of the upstream DHCP library revealed that the convenience helper still called a constructor that generated a random transaction ID before modifiers were applied.
10. The code was rewritten to drive the DHCP handshake explicitly with a deterministic transaction ID.
11. The guest immediately produced DHCP Discover, Offer, Request, and ACK, configured IPv4 successfully, and started serving the webpage.

### Root Cause Analysis

Primary root cause:

- Hidden entropy dependency in the upstream DHCP helper path.

Detailed mechanism:

- `nclient4.Request(...)` eventually calls `dhcpv4.NewDiscovery(...)`.
- `dhcpv4.NewDiscovery(...)` internally calls `dhcpv4.New(...)`.
- `dhcpv4.New(...)` generates a random transaction ID before any modifiers are applied.
- Passing `dhcpv4.WithTransactionID(xid)` did not remove this dependency; it only overwrote the transaction ID after the hidden random allocation had already happened.

Why this mattered here:

- The guest is a tiny initramfs environment with no long-lived entropy accumulation path.
- Early in boot, blocking randomness may be unavailable or delayed.
- A hidden `getrandom` dependency in packet construction is therefore operationally unsafe.

Contributing factors:

- The helper API looked safe because it accepted a transaction ID modifier.
- The process already had several plausible failure candidates:
  - missing NIC driver,
  - missing kernel DHCP support,
  - raw socket misconfiguration,
  - netlink route/address bugs.
- Without packet capture and lower-level logging, those failure modes were easy to conflate.

### Impact

User-visible impact:

- The host could not reliably reach the guest webpage.
- The demo appeared to boot but failed at a critical systems-initialization step.

Engineering impact:

- Multiple debugging iterations were required to separate kernel limitations, QEMU network behavior, socket-layer behavior, and constructor-layer behavior.

### Evidence

Failure-phase evidence:

```text
networking: requesting DHCP lease on eth0 xid=0xbac0e533 deadline=45s
networking: still waiting for DHCP on eth0 xid=0xbac0e533 elapsed=5.008s
...
networking: DHCP wait on eth0 xid=0xbac0e533 ended via context after 44.997s err=context deadline exceeded
```

At that point, packet capture showed no DHCP packets:

```text
$ tshark -r /tmp/qemu-net.pcap -Y 'bootp || dhcp'
# no output
```

Success-phase evidence:

```text
2026/03/08 20:20:11.859805 networking: dhcp-raw(eth0) write start bytes=300 addr=255.255.255.255:67
2026/03/08 20:20:11.925482 dhcp: sent message DHCPv4 Message ... DHCP Message Type: DISCOVER
2026/03/08 20:20:11.934069 dhcp: received message DHCPv4 Message ... DHCP Message Type: OFFER
2026/03/08 20:20:11.950242 dhcp: sent message DHCPv4 Message ... DHCP Message Type: REQUEST
2026/03/08 20:20:11.971370 dhcp: received message DHCPv4 Message ... DHCP Message Type: ACK
2026/03/08 20:20:12.345060 networking: configured eth0 with 10.0.2.15/24 gateway=10.0.2.2 dns=10.0.2.3
2026/03/08 20:20:12.463390 go init ready on :8080
```

Packet capture in the success case:

```text
0.0.0.0 -> 255.255.255.255 DHCP Discover
10.0.2.2 -> 255.255.255.255 DHCP Offer
0.0.0.0 -> 255.255.255.255 DHCP Request
10.0.2.2 -> 255.255.255.255 DHCP ACK
```

### What Fixed It

Code-level changes:

- use `nclient4.NewRawUDPConn(...)` for interface-bound raw DHCP packet IO,
- wrap the packet connection with logging to expose `WriteTo` and `ReadFrom`,
- avoid `nclient4.Request(...)`,
- build Discover and Request packets manually with a deterministic transaction ID,
- keep the same transaction ID across Discover and Request,
- apply address and routes only after ACK decode succeeds.

Relevant code:

- [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go)
- [internal/networking/network_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network_test.go)

### What We Should Do For Real Entropy In A Production Project

If entropy matters for your real project, the current deterministic-xid workaround should be treated as a tactical fix for early boot, not a general randomness strategy.

You need an explicit entropy plan at four layers:

1. QEMU / virtual hardware layer

- Add a virtual RNG device to the guest.
- In QEMU this usually means a `virtio-rng` device backed by a host entropy source.
- Example direction:

```text
-object rng-random,id=rng0,filename=/dev/urandom
-device virtio-rng-pci,rng=rng0
```

What this buys you:

- the guest kernel gets an actual virtual hardware RNG source,
- early boot entropy becomes much less fragile,
- userspace `getrandom` calls are less likely to block indefinitely.

2. Guest kernel layer

- Ensure the guest kernel has the relevant random and virtio RNG support enabled.
- For a custom kernel, verify the equivalent of:
  - `CONFIG_HW_RANDOM`
  - `CONFIG_HW_RANDOM_VIRTIO`
  - `CONFIG_VIRTIO`
  - `CONFIG_VIRTIO_PCI`

What this buys you:

- the guest can consume the virtual RNG device,
- the kernel entropy pool gets seeded earlier and more predictably.

3. Boot persistence / seed handoff layer

- Persist a random seed across boots and feed it back into the kernel or early userspace at startup.
- On a fuller distro this is often done via a seed file under `/var/lib/systemd/random-seed` or equivalent.
- In a minimal initramfs project, you need your own policy if the system has writable persistent storage.

What this buys you:

- the second and later boots are much less entropy-starved,
- you are not depending solely on “fresh” device noise every boot.

4. Userspace blocking policy

- Identify all codepaths that may implicitly call `getrandom` or crypto-grade RNG during early boot.
- Decide which of them must be:
  - blocked until real entropy is available,
  - deferred until later boot,
  - replaced with deterministic or non-cryptographic identifiers for non-security-sensitive uses.

For your project, separate these classes:

- Security-sensitive randomness:
  - session keys
  - TLS keys
  - token generation
  - long-lived secrets

These must use real entropy and should not silently downgrade.

- Operational identifiers that only need uniqueness:
  - DHCP transaction IDs
  - temporary correlation IDs
  - boot-local request IDs

These can use deterministic or weakly-random fallbacks if the protocol does not require cryptographic unpredictability.

### Recommended Entropy Requirements For Your Project

If this project grows beyond a demo, the minimum sane plan is:

- add `virtio-rng` to the QEMU launch path,
- make sure the guest kernel supports `virtio-rng`,
- audit early-boot libraries for hidden blocking randomness,
- keep deterministic fallbacks only for non-secret protocol fields,
- never use deterministic boot-local randomness for secrets,
- add a startup self-check that reports whether the system believes a hardware or virtual RNG is present.

### Design Rules Going Forward

- Do not assume a modifier-based API removes hidden randomness or IO; read the constructor path.
- For early boot, treat randomness as an infrastructure dependency, not an incidental helper.
- Keep packet capture and low-level socket logs available for networking work in initramfs environments.
- Add tests for protocol packet builders whenever the high-level helper path is bypassed.

## Usage Examples

Use this document when:

- an engineer wants to understand why the DHCP path was rewritten manually,
- a future refactor proposes switching back to the convenience helper,
- the project is adding TLS, credentials, or cryptographic setup to early boot,
- someone asks whether QEMU and the guest need an explicit entropy device.

Review sequence:

1. Read [internal/networking/network.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/networking/network.go).
2. Read the diary entry in [01-diary.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/reference/01-diary.md).
3. Reproduce with the playbook in [01-dhcp-packet-capture-and-inspection-playbook.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/playbook/01-dhcp-packet-capture-and-inspection-playbook.md).
4. Use the entropy requirements section above when designing any new early-boot secret-generation path.

## Related

- [01-userspace-dhcp-architecture-and-implementation-guide-for-the-go-init-runtime.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/design-doc/01-userspace-dhcp-architecture-and-implementation-guide-for-the-go-init-runtime.md)
- [02-implementation-guide.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/reference/02-implementation-guide.md)
- [01-diary.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/reference/01-diary.md)
- [01-dhcp-packet-capture-and-inspection-playbook.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/playbook/01-dhcp-packet-capture-and-inspection-playbook.md)
