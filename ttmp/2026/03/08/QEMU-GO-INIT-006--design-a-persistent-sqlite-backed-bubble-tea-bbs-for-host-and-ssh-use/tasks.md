# Tasks

## In Progress

- [ ] Update ticket `006` docs to reflect shared host-directory persistence via QEMU pass-through instead of storing the BBS database only inside `build/data.img`.
- [ ] Implement a shared-state mount path in the guest using QEMU `9p`, including initramfs module packaging and guest mount logic.
- [ ] Add a SQLite-backed BBS store package using `modernc.org/sqlite` and initialize the schema automatically.
- [ ] Add a reusable Bubble Tea BBS application package that can run both on a host TTY and over a Wish SSH session.
- [ ] Add a host `cmd/bbs` binary that opens the same shared-state directory and launches the BBS locally.
- [ ] Replace the current SSH transcript app with the Bubble Tea BBS experience.
- [ ] Add host and guest validation paths, update the diary continuously, and publish the ticket bundle to reMarkable.

## Notes

- `virtiofs` remains the preferred long-term transport, but the current environment does not provide `virtiofsd`.
- For this ticket, use `9p` as the concrete shared-directory implementation and document the concurrency limitations around simultaneous host and guest writes to the same SQLite database.
