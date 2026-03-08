# Tasks

## TODO

- [x] Document the DHCP problem statement, current-state architecture, and target userspace-networking design
- [x] Add Go dependencies for DHCP and Linux netlink configuration
- [x] Implement interface discovery, link-up, DHCP lease acquisition, and IPv4 route/address programming in the init runtime
- [x] Extend the web status API and page to expose network state and lease details
- [x] Update build/run/smoke automation to exercise the userspace DHCP path
- [x] Commit the ticket scaffold and planning artifacts
- [x] Commit the DHCP/runtime implementation and validation changes
- [x] Increase runtime logging around DHCP, link state, and fallback behavior
- [x] Create and refine a packet-capture/inspection playbook for DHCP debugging
- [x] Validate the new logging and packet-capture workflow against `/tmp/qemu-vmlinuz`
- [x] Finalize the detailed design doc, implementation guide, diary, and ticket bookkeeping
- [x] Validate with unit tests plus an end-to-end QEMU boot against `/tmp/qemu-vmlinuz`
- [ ] Run `docmgr doctor` and upload the bundle to reMarkable
