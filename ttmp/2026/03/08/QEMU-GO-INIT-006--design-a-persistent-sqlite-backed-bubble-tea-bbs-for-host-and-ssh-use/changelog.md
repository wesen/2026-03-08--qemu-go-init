# Changelog

## 2026-03-08

- Initial workspace created.
- Probed the proposed stack with a ticket-local Go module and confirmed that `modernc.org/sqlite`, Bubble Tea, and Lip Gloss all build and run with `CGO_ENABLED=0`.
- Confirmed that the Charmbracelet stack does not require external ncurses userland in the guest, but it does include Go-level terminal capability logic via `termenv`, `colorprofile`, and `xo/terminfo`.
- Updated the storage plan from "SQLite only inside guest raw ext4 image" to "shared host directory mounted into the guest", because the latter supports a host-native BBS binary using the same content store.
- Verified that `virtiofs` is not immediately usable in this environment because `virtiofsd` is absent, while the current kernel does provide `9p` support as loadable modules.
