# Changelog

## 2026-03-08

- Initial workspace created
- Scoped the ticket around replacing external SSH userland with an in-process Wish-based SSH app hosted by the Go init runtime
- Confirmed the current upstream module path and local module resolution with a ticket-scoped probe using `github.com/charmbracelet/wish v1.4.7`
- Validated that a minimal Wish server can self-host SSH locally, auto-generate a host key, accept auth-less local probe connections by default, and expose PTY-vs-non-PTY behavior through middleware
- `docmgr doctor --ticket QEMU-GO-INIT-004 --stale-after 30` passed after adding `ssh` to the vocabulary topics
- Uploaded the ticket bundle to reMarkable at `/ai/2026/03/08/QEMU-GO-INIT-004` and verified the remote listing
- Added `internal/sshapp` with Wish-backed config loading, lifecycle management, session rendering, and focused unit coverage (commit `b124f52`)
- Wired the Wish service into the real boot path so PID 1 now starts HTTP and SSH together using a stable guest host-key path (commit `61fc6c8`)
- Exposed SSH status through `/api/status` and the embedded operator webpage (commit `af961f5`)
- Added SSH host forwarding to the QEMU workflow and validated the live guest through automated SSH smoke coverage (commit `efb942d`)
- Re-ran `docmgr doctor`, refreshed the ticket diary/tasks, and force-updated the reMarkable bundle after implementation completion
