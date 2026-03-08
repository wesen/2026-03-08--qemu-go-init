# Tasks

## TODO

- [x] Create the ticket workspace, primary design doc, implementation guide, and diary
- [x] Research Wish from primary sources and confirm the current import path and API surface
- [x] Run a local Wish probe in the ticket `scripts/` directory to validate server startup and session behavior
- [x] Document the current repo integration points and constraints for adding an SSH app to the Go init runtime
- [x] Write a detailed architecture and implementation guide for a new intern
- [x] Run `docmgr doctor` and upload the ticket bundle to reMarkable
- [ ] Add `internal/sshapp` with Wish-backed server startup, config loading, status reporting, and session rendering
- [ ] Wire the SSH service into the PID 1 boot sequence and use a stable guest host-key path
- [ ] Expose SSH state in `/api/status` and the embedded webpage
- [ ] Extend the QEMU run/smoke tooling with SSH port forwarding and host-side validation
- [ ] Validate the full HTTP + SSH guest workflow and refresh the ticket docs, diary, and bundle
