# Changelog

## 2026-03-09

- Initial workspace created
- Defined the migration target: packaged dynamic CGO guest runtime, upstream Pinocchio turn and timeline stores, guest log persistence in SQLite, and host-side QEMU log capture/import.

## 2026-03-09

Documented the CGO migration architecture, added the runtime probe script, and anchored the ticket to the upstream Pinocchio turn and timeline persistence hooks.

### Related Files

- /home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/09/QEMU-GO-INIT-009--migrate-guest-chat-persistence-and-logging-to-upstream-cgo-backed-sqlite-stores/design-doc/01-cgo-backed-sqlite-persistence-and-runtime-packaging-plan-for-qemu-go-init.md — Primary architecture and migration plan
- /home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/09/QEMU-GO-INIT-009--migrate-guest-chat-persistence-and-logging-to-upstream-cgo-backed-sqlite-stores/scripts/cgo-runtime-probe.sh — Reusable probe for CGO guest dependencies


## 2026-03-09

Enabled a dynamically linked CGO guest build, added automatic ELF runtime dependency collection, and proved the packaged guest still boots under QEMU smoke.

### Related Files

- /home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile — Switch guest build to CGO and generate runtime dependency maps
- /home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/mkinitramfs/main.go — Add file-map-file support for generated runtime dependency lists
- /home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/collect-elf-runtime.sh — Collect loader and shared library mappings for initramfs packaging

## 2026-03-09

Reused the upstream Pinocchio CGO-backed turn and timeline stores inside the guest chat surface, moved guest chat state onto ext4-backed persistent storage, and added guest SQLite log persistence plus HTTP debug endpoints.

### Related Files

- /home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/aichat/persistence.go — Opens and wires the upstream Pinocchio turn and timeline SQLite stores
- /home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/aichat/surface.go — Connects the turn persister and timeline persistence handler to the chat backend
- /home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go — Moves guest chat state to ext4 storage and initializes the guest SQLite log store
- /home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/logstore/store.go — Local CGO-backed SQLite log store for guest runtime logs
- /home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go — Adds the runtime debug endpoints for chat persistence and guest logs

## 2026-03-09

Added host-side QEMU serial log import, validated guest turn and timeline counts after a real SSH chat turn, and confirmed host QEMU logs land in SQLite.

### Related Files

- /home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/importqemulogs/main.go — Host utility that imports QEMU serial logs into SQLite
- /home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh — Imports host-side QEMU logs after smoke completion
- /home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/aichat/debug.go — Reports runtime row counts for turns and timeline entities without mutating the databases
