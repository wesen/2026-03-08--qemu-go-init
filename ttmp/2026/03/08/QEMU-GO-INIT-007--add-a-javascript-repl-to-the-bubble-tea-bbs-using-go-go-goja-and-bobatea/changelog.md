# Changelog

## 2026-03-08

- Initial workspace created
- Clarified the integration target: keep the BBS as the top-level application and embed a Bobatea-backed JavaScript REPL mode inside it.
- Verified that Wish requires `MiddlewareWithProgramHandler` if the REPL needs access to the concrete `*tea.Program` for `timeline.RegisterUIForwarder`.
- Verified that `go-go-goja` already ships a Bobatea evaluator adapter at `pkg/repl/adapters/bobatea`.
- Added a ticket-local probe at `scripts/js-repl-probe` and verified evaluator + bus + REPL model construction.
- Verified that the local dependency stack requires Go `>= 1.25.7`; `GOTOOLCHAIN=auto` is sufficient in the probe environment.
