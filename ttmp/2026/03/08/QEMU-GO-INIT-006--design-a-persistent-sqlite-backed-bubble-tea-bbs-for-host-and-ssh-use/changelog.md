# Changelog

## 2026-03-08

- Initial workspace created.
- Probed the proposed stack with a ticket-local Go module and confirmed that `modernc.org/sqlite`, Bubble Tea, and Lip Gloss all build and run with `CGO_ENABLED=0`.
- Confirmed that the Charmbracelet stack does not require external ncurses userland in the guest, but it does include Go-level terminal capability logic via `termenv`, `colorprofile`, and `xo/terminfo`.
- Updated the storage plan from "SQLite only inside guest raw ext4 image" to "shared host directory mounted into the guest", because the latter supports a host-native BBS binary using the same content store.
- Verified that `virtiofs` is not immediately usable in this environment because `virtiofsd` is absent, while the current kernel does provide `9p` support as loadable modules.
- Added a generic initramfs module-packaging path so multiple kernel modules can be embedded without special-casing each one.
- Added guest shared-state mount plumbing and QEMU `-virtfs` wiring for a `9p`-backed host-shared directory.
- Added a SQLite-backed store with schema initialization and a seeded welcome post.
- Added a reusable Bubble Tea BBS model plus a host `cmd/bbs` entrypoint.
- Replaced the SSH transcript app with the Bubble Tea BBS using Wish middleware.
- Fixed the guest `9p` mount by bundling and loading the missing `netfs` dependency module.
- Validated the implementation with `go test ./...`, `make smoke`, and a host-side terminal render of `cmd/bbs`.
- Added `sqlite` and `tui` to the docmgr vocabulary so the ticket validates cleanly.
- Uploaded the refreshed ticket bundle to reMarkable at `/ai/2026/03/08/QEMU-GO-INIT-006`.
