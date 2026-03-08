# Tasks

## TODO

- [x] Create the ticket workspace, primary design doc, implementation guide, and diary
- [x] Document the current entropy-related architecture, gaps, and target design for QEMU virtio-rng plus guest diagnostics
- [x] Add QEMU launch support for a `virtio-rng` device in `Makefile` and `scripts/qemu-smoke.sh`
- [x] Add a guest-side entropy diagnostics package that reports entropy availability and RNG-device visibility
- [x] Expose entropy diagnostics in `/api/status` and the embedded webpage
- [x] Validate the new entropy path with unit tests and an end-to-end QEMU smoke boot
- [x] Commit the entropy runtime changes and validation results
- [x] Record the code commits, validation output, and kernel-module blocker in the diary and changelog
- [x] Decide and implement the next kernel-side step needed for actual virtio-rng activation in this module-less initramfs
- [x] Run `docmgr doctor` and upload the ticket bundle to reMarkable
- [x] Add a full-system architecture, usage, and extension guide for the current repository state
