# Changelog

## 2026-03-08

- Initial workspace created
- Clarified the integration target: keep the BBS as the top-level application and embed a Bobatea-backed JavaScript REPL mode inside it.
- Verified that Wish requires `MiddlewareWithProgramHandler` if the REPL needs access to the concrete `*tea.Program` for `timeline.RegisterUIForwarder`.
- Verified that `go-go-goja` already ships a Bobatea evaluator adapter at `pkg/repl/adapters/bobatea`.
- Added a ticket-local probe at `scripts/js-repl-probe` and verified evaluator + bus + REPL model construction.
- Verified that the local dependency stack requires Go `>= 1.25.7`; `GOTOOLCHAIN=auto` is sufficient in the probe environment.
- Added `internal/jsrepl` as the repo-owned integration layer and embedded it into the BBS as a full-screen REPL mode.
- Updated `cmd/bbs` to attach the REPL bus to the concrete `*tea.Program`.
- Updated `internal/sshbbs` to use `wishbubbletea.MiddlewareWithProgramHandler`.
- First QEMU smoke attempt failed because the higher-level `go-go-goja` Bobatea adapter pulled in tree-sitter packages that do not compile under `CGO_ENABLED=0`.
- Pivoted to a CGO-free evaluator built directly on `go-go-goja/engine` while keeping Bobatea as the UI shell.
- Full `go test ./...` passed after the pivot.
- `make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18084 QEMU_SSH_HOST_PORT=10026` passed after the pivot.
- `docmgr doctor --ticket QEMU-GO-INIT-007 --stale-after 30` passed cleanly.
- Uploaded the ticket bundle to reMarkable at `/ai/2026/03/08/QEMU-GO-INIT-007`.
