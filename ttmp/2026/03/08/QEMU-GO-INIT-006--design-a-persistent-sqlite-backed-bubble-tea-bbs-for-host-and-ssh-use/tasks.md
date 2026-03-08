# Tasks

## In Progress

- [x] Update ticket `006` docs to reflect shared host-directory persistence via QEMU pass-through instead of storing the BBS database only inside `build/data.img`.
- [x] Implement a shared-state mount path in the guest using QEMU `9p`, including initramfs module packaging and guest mount logic.
- [x] Add a SQLite-backed BBS store package using `modernc.org/sqlite` and initialize the schema automatically.
- [x] Add a reusable Bubble Tea BBS application package that can run both on a host TTY and over a Wish SSH session.
- [x] Add a host `cmd/bbs` binary that opens the same shared-state directory and launches the BBS locally.
- [x] Replace the current SSH transcript app with the Bubble Tea BBS experience.
- [x] Add host and guest validation paths, update the diary continuously, and publish the ticket bundle to reMarkable.

## Notes

- `virtiofs` remains the preferred long-term transport, but the current environment does not provide `virtiofsd`.
- For this ticket, use `9p` as the concrete shared-directory implementation and document the concurrency limitations around simultaneous host and guest writes to the same SQLite database.
- Current code paths:
  - host CLI: [cmd/bbs/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/bbs/main.go)
  - shared-state mount: [internal/sharedstate/sharedstate.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sharedstate/sharedstate.go)
  - BBS store: [internal/bbsstore/store.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/bbsstore/store.go)
  - Bubble Tea model: [internal/bbsapp/model.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/bbsapp/model.go)
  - SSH adapter: [internal/sshbbs/middleware.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshbbs/middleware.go)
