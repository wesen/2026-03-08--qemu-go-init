# Tasks

## TODO

- [x] Document the current runtime, the upstream persistence hooks, and the CGO packaging plan for a new intern.
- [x] Add a ticket-local probe script that builds a CGO guest binary and reports its dynamic library dependencies.
- [x] Teach the guest build and initramfs pipeline to build a CGO `/init` and package the ELF interpreter plus all required shared libraries.
- [x] Validate that the dynamic `/init` boots successfully in QEMU and record the exact packaged runtime dependencies.
- [x] Add a persistent chat runtime state directory layout for turns, timeline snapshots, and logs.
- [x] Reuse the upstream Pinocchio SQLite turn store for final-turn persistence in the guest chat backend.
- [x] Reuse the upstream Pinocchio SQLite timeline store and wire `StepTimelinePersistFuncWithVersion` into the guest chat router.
- [x] Add guest application log persistence to SQLite, including zerolog output and the stdlib logger path used by PID 1.
- [x] Expose HTTP debug status for chat persistence and guest log persistence.
- [x] Add a host-side capture path for QEMU serial logs and an import step into SQLite.
- [x] Validate end-to-end: boot, chat, persisted turns, persisted timeline rows, persisted guest logs, captured QEMU logs.
- [x] Update ticket docs, changelog, diary, and reMarkable upload after each significant slice.
