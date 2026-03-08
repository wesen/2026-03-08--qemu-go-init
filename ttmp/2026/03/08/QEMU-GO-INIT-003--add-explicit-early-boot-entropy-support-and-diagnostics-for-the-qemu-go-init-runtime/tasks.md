# Tasks

## TODO

- [x] Create the ticket workspace, primary design doc, implementation guide, and diary
- [ ] Document the current entropy-related architecture, gaps, and target design for QEMU virtio-rng plus guest diagnostics
- [ ] Add QEMU launch support for a `virtio-rng` device in `Makefile` and `scripts/qemu-smoke.sh`
- [ ] Add a guest-side entropy diagnostics package that reports entropy availability and RNG-device visibility
- [ ] Expose entropy diagnostics in `/api/status` and the embedded webpage
- [ ] Validate the new entropy path with unit tests and an end-to-end QEMU smoke boot
- [ ] Commit the entropy runtime changes and validation results
- [ ] Update the diary, changelog, and file relationships with the code commit
- [ ] Run `docmgr doctor` and upload the ticket bundle to reMarkable
