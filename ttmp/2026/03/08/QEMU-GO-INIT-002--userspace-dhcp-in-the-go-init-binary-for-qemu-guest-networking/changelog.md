# Changelog

## 2026-03-08

- Initial workspace created
- Scoped the ticket around userspace DHCP inside the Go `/init` binary after confirming the current kernel ignores `ip=dhcp`
- Added the userspace networking package, DHCP/netlink dependencies, UI status wiring, and QEMU automation updates
- Validated unit tests and iterated on guest-networking experiments, including entropy-safe DHCP transaction IDs and alternate socket strategies
- Added a packet-capture playbook to debug the remaining DHCP client path
- Added DHCP watchdog logging, interface/state logging, and a bounded `timeout 75s` reproduction flow that proves the guest enters DHCP while no BOOTP packets appear in the QEMU capture
- Replaced the helper-based DHCP request path with a deterministic-xid raw-socket handshake, which restored full DORA exchange and a working host-visible web server under QEMU user networking
