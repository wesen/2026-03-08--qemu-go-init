# Changelog

## 2026-03-08

- Initial workspace created
- Scoped the ticket around userspace DHCP inside the Go `/init` binary after confirming the current kernel ignores `ip=dhcp`
- Added the userspace networking package, DHCP/netlink dependencies, UI status wiring, and QEMU automation updates
- Validated unit tests and iterated on guest-networking experiments, including entropy-safe DHCP transaction IDs and alternate socket strategies
- Added a packet-capture playbook to debug the remaining DHCP client path
- Added DHCP watchdog logging, interface/state logging, and a bounded `timeout 75s` reproduction flow that proves the guest enters DHCP while no BOOTP packets appear in the QEMU capture
- Replaced the helper-based DHCP request path with a deterministic-xid raw-socket handshake, which restored full DORA exchange and a working host-visible web server under QEMU user networking

## 2026-03-08

Added a detailed postmortem for the early-boot DHCP entropy stall, including production guidance for QEMU entropy devices, guest kernel support, seed persistence, and userspace blocking policy

### Related Files

- /home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/reference/03-postmortem-early-boot-dhcp-entropy-stall-and-recovery.md — Postmortem and entropy guidance

