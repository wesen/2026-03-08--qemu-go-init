# Tasks

## TODO

- [x] Confirm local package APIs and execution model for `go-go-goja` and `bobatea`.
- [x] Create a ticket-local probe under `scripts/` that instantiates the JS evaluator and Bobatea REPL shell.
- [ ] Add repo-local Go module wiring for `github.com/go-go-golems/go-go-goja` and `github.com/go-go-golems/bobatea`.
- [ ] Add a reusable integration package that owns the JS evaluator, Bobatea bus, and program attachment lifecycle.
- [ ] Refactor the BBS app to support a dedicated REPL mode and mode-switching UX.
- [ ] Attach the REPL-enabled BBS in the host CLI path.
- [ ] Attach the REPL-enabled BBS in the Wish SSH path using a custom program handler.
- [ ] Validate host-native and SSH execution paths.
- [ ] Update docs, diary, changelog, and upload the final bundle to reMarkable.
