# Changelog

## 2026-03-08

- Initial workspace created
- Scoped the ticket around replacing external SSH userland with an in-process Wish-based SSH app hosted by the Go init runtime
- Confirmed the current upstream module path and local module resolution with a ticket-scoped probe using `github.com/charmbracelet/wish v1.4.7`
- Validated that a minimal Wish server can self-host SSH locally, auto-generate a host key, accept auth-less local probe connections by default, and expose PTY-vs-non-PTY behavior through middleware
- `docmgr doctor --ticket QEMU-GO-INIT-004 --stale-after 30` passed after adding `ssh` to the vocabulary topics
- Uploaded the ticket bundle to reMarkable at `/ai/2026/03/08/QEMU-GO-INIT-004` and verified the remote listing
- Added `internal/sshapp` with Wish-backed config loading, lifecycle management, session rendering, and focused unit coverage (commit `b124f52`)
