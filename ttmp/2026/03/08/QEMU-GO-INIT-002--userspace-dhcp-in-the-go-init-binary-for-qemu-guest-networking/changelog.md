# Changelog

## 2026-03-08

- Initial workspace created
- Scoped the ticket around userspace DHCP inside the Go `/init` binary after confirming the current kernel ignores `ip=dhcp`
- Added the userspace networking package, DHCP/netlink dependencies, UI status wiring, and QEMU automation updates
- Validated unit tests and iterated on guest-networking experiments, including entropy-safe DHCP transaction IDs and alternate socket strategies
- Added a packet-capture playbook to debug the remaining DHCP client path
