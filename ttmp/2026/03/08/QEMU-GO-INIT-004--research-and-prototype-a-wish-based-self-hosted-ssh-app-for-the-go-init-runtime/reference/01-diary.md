---
Title: Diary
Ticket: QEMU-GO-INIT-004
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - ssh
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/go.mod
      Note: step 2 module-resolution evidence for current Wish version
    - Path: ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/main.go
      Note: step 2 local probe implementation and observed PTY behavior
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T17:20:24.590178225-04:00
WhatFor: ""
WhenToUse: ""
---


# Diary

## Goal

Capture the research, experiment results, and documentation work for evaluating a Wish-based SSH application inside the current Go PID 1 runtime.

## Step 1: Create the Wish SSH research ticket and scope the work

This step turned the user’s request into a concrete research ticket with a bounded deliverable: determine whether Wish can self-host SSH inside the guest without a separate SSH daemon, and document a realistic integration plan for the current repo. The key scope decision was to treat this as an architecture and prototype ticket, not as a full implementation ticket yet.

That distinction matters because “full SSH server” is ambiguous. Before recommending any code path, the ticket needed to separate “Wish can host SSH sessions in-process” from “the repo should behave like a full shell-oriented OpenSSH environment.”

### Prompt Context

**User prompt (verbatim):** "Now create a new ticket and let's make a fun little ssh app using https://github.com/charmbracelet/wish which should allow us to self host a full ssh server without any userland. Create the new ticket, research and create a detailed implitation guide. You can run experiments in scripts/ of the ticket. Keep a detailed diary as you go."

**Assistant interpretation:** Create a new ticket for a Wish-based SSH app, research the upstream API and fit with the current repo, run experiments in the ticket workspace, and document the results in a detailed implementation guide and diary.

**Inferred user intent:** Determine whether the existing single-binary init runtime can grow into a self-hosted SSH application without importing a traditional SSH daemon or broad guest userland.

**Commit (code):** acabe28ddd7c2cdcbc794263a286f69fb8702839 — "Add Wish SSH research ticket and probe"

### What I did

- Created ticket `QEMU-GO-INIT-004`.
- Added:
  - a primary design doc,
  - an implementation guide,
  - a diary.
- Scoped the ticket around Wish-based SSH hosting for the current Go PID 1 runtime.

### Why

- The question is architectural first. A clear ticket boundary prevents premature implementation of an SSH path with the wrong auth model or wrong product assumptions.

### What worked

- The ticket scaffold was created cleanly and gives the research a dedicated workspace.

### What didn't work

- N/A

### What I learned

- The most important open question is not “can Wish listen on an SSH socket?” but “what behavior do we want clients to experience once they connect?”

### What was tricky to build

- The tricky part here was scope control. “SSH server” can imply very different outcomes: a custom SSH app, a PTY-only TUI, an exec router, or a full shell service.

### What warrants a second pair of eyes

- The final implementation ticket should explicitly choose one product shape for phase 1 instead of mixing all SSH behaviors together.

### What should be done in the future

- Run a local compile-and-connect probe against Wish before making any repo-level code recommendations.

### Code review instructions

- Review:
  - [tasks.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/tasks.md)
  - [01-wish-based-ssh-app-architecture-analysis-and-implementation-guide-for-the-go-init-runtime.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/design-doc/01-wish-based-ssh-app-architecture-analysis-and-implementation-guide-for-the-go-init-runtime.md)

### Technical details

Commands used:

```bash
docmgr ticket create-ticket --ticket QEMU-GO-INIT-004 --title "Research and prototype a Wish-based self-hosted SSH app for the Go init runtime" --topics go,qemu,linux,initramfs,ssh
docmgr doc add --ticket QEMU-GO-INIT-004 --doc-type design-doc --title "Wish-based SSH app architecture, analysis, and implementation guide for the Go init runtime"
docmgr doc add --ticket QEMU-GO-INIT-004 --doc-type reference --title "Diary"
docmgr doc add --ticket QEMU-GO-INIT-004 --doc-type reference --title "Implementation guide"
```

## Step 2: Run a local Wish probe and validate what the library actually does today

This step created a small isolated Go module under the ticket’s `scripts/` directory and used it to answer the questions that matter for implementation: current import path, current module version, host key behavior, authentication defaults, and PTY-vs-non-PTY session behavior.

The probe was valuable because it corrected one assumption immediately. Wish v1.4.7 uses `github.com/charmbracelet/ssh` in the effective server API, not the plain `gliderlabs/ssh` import I initially wrote. The probe also exposed the practical effect of `activeterm.Middleware()`: non-PTY sessions fail with `Requires an active PTY`, which makes it unsuitable as a blanket middleware if exec-style sessions are desired.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Validate the actual current Wish API and runtime behavior with a ticket-local experiment before writing the integration guide.

**Inferred user intent:** Base the design on real current library behavior, not on stale memory or optimistic assumptions.

**Commit (code):** acabe28ddd7c2cdcbc794263a286f69fb8702839 — "Add Wish SSH research ticket and probe"

### What I did

- Added:
  - [scripts/wish-probe/go.mod](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/go.mod)
  - [scripts/wish-probe/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/main.go)
- Ran `go mod tidy`, `go build ./...`, and `go doc` against the probe module.
- Started the Wish server locally and connected with the host `ssh` client in both non-PTY and PTY modes.

### Why

- The local probe is the fastest way to answer library-shape and session-behavior questions without touching the main repo runtime.

### What worked

- `go mod tidy` resolved:

```text
github.com/charmbracelet/wish v1.4.7
github.com/charmbracelet/ssh v0.0.0-20250128164007-98fd5ae11894
```

- `go build ./...` passed after switching the app import from `github.com/gliderlabs/ssh` to `github.com/charmbracelet/ssh`.
- Starting the probe generated a host key automatically and listened successfully:

```text
2026/03/08 17:24:33 wish probe: listening on 127.0.0.1:22230 hostkey=.wish_probe_ed25519
```

- A PTY-backed connection succeeded and printed session metadata:

```text
wish probe ready
user=manuel
remote=127.0.0.1:47230
command=[]
has_pty=true
term=dumb size=0x0
resize=0x0
goodbye
```

### What didn't work

- The first build failed with a type mismatch:

```text
cannot use func(next ssh.Handler) ssh.Handler {…} (value of type func(next "github.com/gliderlabs/ssh".Handler) "github.com/gliderlabs/ssh".Handler) as wish.Middleware value in return statement
```

- A non-PTY connection succeeded through the custom handler but then failed because `activeterm.Middleware()` rejected it:

```text
Requires an active PTY
```

### What I learned

- Current Wish resolves locally as `github.com/charmbracelet/wish v1.4.7`.
- Wish server startup is easy enough for this repo: `wish.NewServer` plus address, host key, and middleware options.
- PTY policy is a product decision, not a harmless default.

### What was tricky to build

- The tricky part was not compiling the server. It was interpreting what the middleware stack means for the eventual product shape. A PTY-oriented middleware chain is correct for a TUI app and incorrect for a general exec endpoint.

### What warrants a second pair of eyes

- The distinction between:
  - custom interactive SSH app,
  - SSH exec router,
  - shell-like SSH service.
- Those should remain separate product choices in any implementation ticket.

### What should be done in the future

- Use this probe as the template for any future Wish API validation before upgrading versions or changing middleware assumptions.

### Code review instructions

- Review:
  - [scripts/wish-probe/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/main.go)
  - [scripts/wish-probe/go.mod](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe/go.mod)
- Re-run:

```bash
cd ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/scripts/wish-probe
go run .
ssh -tt -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=none -o PubkeyAuthentication=no -o PasswordAuthentication=no -p 22230 localhost
```

### Technical details

Key commands:

```bash
go mod tidy
go build ./...
go doc github.com/charmbracelet/wish
go doc github.com/charmbracelet/wish.Middleware
go doc github.com/charmbracelet/wish.WithAuthorizedKeys
go doc github.com/charmbracelet/wish/activeterm
ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=none -o PubkeyAuthentication=no -o PasswordAuthentication=no -p 22230 localhost true
timeout 5s ssh -tt -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=none -o PubkeyAuthentication=no -o PasswordAuthentication=no -p 22230 localhost
go list -m all | rg 'charmbracelet/wish|charmbracelet/ssh|gliderlabs/ssh'
```

Observed module list excerpt:

```text
github.com/charmbracelet/ssh v0.0.0-20250128164007-98fd5ae11894
github.com/charmbracelet/wish v1.4.7
github.com/gliderlabs/ssh v0.3.8
```

## Step 4: Build the standalone Wish service package before touching PID 1

This step implemented the first execution task from the updated ticket: create a repo-native `internal/sshapp` package that can load configuration from the environment, start a Wish-backed SSH server, track status, and render a simple interactive session without yet changing the boot sequence. The important part of this slice was keeping it self-contained enough to test in isolation before any QEMU or PID 1 wiring entered the picture.

That isolation paid off immediately. The first broad `go test` run appeared to hang, so I narrowed the scope to the live server test with an explicit timeout. The issue turned out not to be a package-level deadlock, but a lifecycle ambiguity in the test loop. Once I reran the focused test directly, the package behaved as intended and the server startup/shutdown path proved stable.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Start implementing the Wish ticket task by task, with commit boundaries and diary updates after each meaningful slice.

**Inferred user intent:** Land the SSH feature incrementally, so each step is verifiable and well documented rather than bundled into one opaque change.

**Commit (code):** b124f52 — "Add Wish SSH service package"

### What I did

- Added:
  - [internal/sshapp/config.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/config.go)
  - [internal/sshapp/server.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/server.go)
  - [internal/sshapp/server_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/server_test.go)
- Added the Wish dependency to the main module with `go get github.com/charmbracelet/wish@v1.4.7`.
- Implemented:
  - env-based SSH config,
  - a Wish server lifecycle wrapper,
  - live status reporting,
  - a simple SSH session transcript renderer,
  - focused tests for config parsing, session rendering, and start/shutdown behavior.

### Why

- The service package is the safest seam to validate first because it exercises the new dependency and the SSH lifecycle without entangling the existing PID 1 boot logic.

### What worked

- `go get github.com/charmbracelet/wish@v1.4.7 && go mod tidy`
- `go test ./internal/sshapp -count=1`
- A real local Wish listener could be started and shut down from the unit test with a generated host key under a temp directory.

### What didn't work

- My first broad package test invocation appeared to hang with no output:

```text
go test ./internal/sshapp -count=1
```

- I treated that as a lifecycle problem and reran the live server test explicitly with a timeout:

```text
go test ./internal/sshapp -run TestStartAndShutdown -count=1 -v -timeout 8s
```

- That isolated run passed cleanly, which showed the package was viable and the earlier hang was not a deterministic service deadlock.

### What I learned

- The clean way to integrate Wish here is as a small lifecycle wrapper around `wish.NewServer` plus `net.Listen` and `server.Serve`, rather than delegating startup entirely to `ListenAndServe`.
- Tracking host-key presence and live session counters in the service layer will make the later web UI work straightforward.

### What was tricky to build

- The tricky part was deciding how much of the upstream SSH session interface the renderer should depend on. Making the renderer consume only the subset it actually uses avoided a brittle test double and kept the session output unit-testable.

### What warrants a second pair of eyes

- The auth model is intentionally minimal right now: the service uses Wish’s no-auth behavior by leaving password and public-key handlers unset. That is correct for the demo slice, but it is intentionally not production-ready.
- The session currently renders status text and exits, which is a good fit for this phase but not yet a richer TUI or command router.

### What should be done in the future

- Wire the service into [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go), expose its status through the web surface, and then add QEMU SSH forwarding and smoke validation.

### Code review instructions

- Start with:
  - [internal/sshapp/server.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/server.go)
  - [internal/sshapp/config.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/config.go)
  - [internal/sshapp/server_test.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/server_test.go)
- Re-run:

```bash
go test ./internal/sshapp -count=1
```

### Technical details

Commands used:

```bash
go get github.com/charmbracelet/wish@v1.4.7 && go mod tidy
gofmt -w internal/sshapp/config.go internal/sshapp/server.go internal/sshapp/server_test.go
go test ./internal/sshapp -count=1
go test ./internal/sshapp -run TestStartAndShutdown -count=1 -v -timeout 8s
```

## Step 5: Start the Wish service from the actual PID 1 boot flow

This step moved the SSH work out of the isolated package boundary and into the real runtime. The Go init process now starts the Wish service after networking is configured and before the embedded HTTP server begins serving requests. That means the guest is no longer merely SSH-capable in a test package; it now boots both services in the live VM runtime.

The implementation stays deliberately narrow. I did not touch the web UI or QEMU smoke flow yet. The only goal here was to make PID 1 own the SSH service lifecycle and to use a stable guest-side host-key path that will also make sense once persistence lands later.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Continue executing the ticket task list incrementally, keeping the boot-path change separate from the UI and host-tooling work.

**Inferred user intent:** Make the SSH app real inside the guest runtime, not just a library experiment.

**Commit (code):** 61fc6c8 — "Start Wish SSH app during boot"

### What I did

- Updated [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go) to:
  - load SSH config from env,
  - start the Wish service after networking,
  - pass a runtime snapshot into the session renderer,
  - fail the boot if the SSH service cannot start,
  - log both the HTTP and SSH listener addresses once ready.
- Used the default host-key path from the new package:
  - `/var/lib/go-init/ssh/ssh_host_ed25519`

### Why

- Starting the service from PID 1 is the real feature boundary. Until this step, the repo only had a reusable SSH package, not a booted SSH service.

### What worked

- `gofmt -w cmd/init/main.go`
- `go test ./... -count=1`

### What didn't work

- N/A

### What I learned

- The existing boot sequence was already clean enough that the SSH lifecycle could slot in without structural refactoring.
- Using a fixed guest path for the host key now avoids churn later when persistent storage is added.

### What was tricky to build

- The tricky part was deciding what data to hand to the SSH session renderer without coupling the SSH package too tightly to the rest of the repo. Passing a narrow snapshot struct from `main` preserved that boundary while still letting the SSH app describe the live system.

### What warrants a second pair of eyes

- Because SSH now starts by default as part of boot, any future port-binding failure on `:2222` will halt the guest. That is the right behavior for an enabled core service, but it raises the importance of good host-side smoke coverage in the next step.

### What should be done in the future

- Surface SSH status in the status API and UI, then add QEMU host forwarding and a real end-to-end smoke test.

### Code review instructions

- Start with [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go).
- Re-run:

```bash
go test ./... -count=1
```

### Technical details

Commands used:

```bash
gofmt -w cmd/init/main.go
go test ./... -count=1
```

## Step 6: Surface SSH state in the JSON API and embedded webpage

This step made the new SSH service visible to operators without requiring an SSH client first. The existing browser-facing status surface was already the fastest way to inspect mounts, networking, and entropy. Extending that same surface to include SSH state preserved a single operational control plane and made it much easier to see whether the service was enabled, listening, and generating its host key correctly.

The implementation stayed intentionally dynamic. Instead of freezing SSH state at handler-construction time, the web layer now accepts a status callback so the page can reflect live session counters and the actual listener address over time.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Continue the implementation task list by making the new SSH runtime observable from the existing web/API surfaces.

**Inferred user intent:** Keep the system inspectable and easy to debug while the SSH feature is being integrated.

**Commit (code):** af961f5 — "Expose SSH status in web UI"

### What I did

- Updated [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go) to accept an SSH status callback and include an `ssh` object in `/api/status`.
- Updated [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html) to render an SSH panel with:
  - listen address,
  - host-key presence,
  - auth mode,
  - PTY policy,
  - session counters,
  - service error state.
- Updated [cmd/init/main.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/cmd/init/main.go) to pass the live SSH status function into the web handler.

### Why

- Browser-visible status is the quickest way to confirm the guest is running the intended SSH service before trying host-side forwarding and SSH client behavior.

### What worked

- `gofmt -w internal/webui/site.go cmd/init/main.go`
- `go test ./... -count=1`

### What didn't work

- N/A

### What I learned

- The existing web status design already had the right shape for service-level observability. A live callback was enough; no structural rewrite was needed.

### What was tricky to build

- The tricky part was keeping the SSH status live. Passing a `func() sshapp.Status` into the handler preserved dynamic counters and startup state without introducing a global singleton or cross-package back-reference.

### What warrants a second pair of eyes

- The HTML page is now carrying one more runtime panel and more client-side rendering logic. It is still small, but future service additions should watch for the page becoming too monolithic.

### What should be done in the future

- Extend the host-side QEMU tooling so the new UI is paired with an actual SSH smoke path.

### Code review instructions

- Start with:
  - [internal/webui/site.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/site.go)
  - [internal/webui/static/index.html](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/webui/static/index.html)
- Re-run:

```bash
go test ./... -count=1
```

### Technical details

Commands used:

```bash
gofmt -w internal/webui/site.go cmd/init/main.go
go test ./... -count=1
```

## Step 7: Forward SSH from the host and validate the real guest through QEMU smoke

This step closed the loop from implementation to live guest validation. The QEMU run path now forwards an SSH host port into the guest, and the smoke harness exercises both services: HTTP for the status API and Wish over SSH for the interactive transcript. That made the feature concrete enough to answer the practical question “how do I reach it from the host?” with a tested command rather than a design guess.

This step also corrected the middleware ordering. The earlier local probe showed that PTY enforcement can happen after app output if the chain is ordered carelessly. Reordering the middleware list so `activeterm` executes before the session renderer made the intended PTY policy deterministic.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Finish the remaining host-exposure and validation tasks by forwarding SSH through QEMU and proving the full workflow end to end.

**Inferred user intent:** Make the feature usable from the host machine, not just internally correct in the guest codebase.

**Commit (code):** efb942d — "Add SSH port forwarding and smoke coverage"

### What I did

- Updated [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile) to forward a host SSH port into guest port `2222`.
- Updated [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh) to:
  - forward both HTTP and SSH,
  - fetch `/api/status`,
  - open a PTY-backed SSH session from the host,
  - assert the Wish transcript markers are present.
- Reordered middleware construction in [internal/sshapp/server.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/server.go) so PTY enforcement happens before the app session renderer.
- Ran a full QEMU boot against `/tmp/qemu-vmlinuz`.

### Why

- Without host forwarding and a smoke test, the SSH service would still be a “looks correct” feature rather than a demonstrated host-reachable feature.

### What worked

- `bash -n scripts/qemu-smoke.sh`
- `go test ./... -count=1`
- `make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 QEMU_SSH_HOST_PORT=10022`

Observed guest/API excerpt:

```json
"ssh": {
  "enabled": true,
  "started": true,
  "listenAddr": "0.0.0.0:2222",
  "hostKeyPath": "/var/lib/go-init/ssh/ssh_host_ed25519",
  "hostKeyPresent": true,
  "requirePty": true,
  "authMode": "none"
}
```

Observed host SSH transcript excerpt:

```text
qemu-go-init / wish
===================
This guest is a single Go PID 1 runtime.
It exposes an SSH app, not a general shell.
```

### What didn't work

- N/A

### What I learned

- The repo’s existing QEMU smoke harness was flexible enough to grow into a dual-protocol validation path with only modest changes.
- The current host entrypoints are now concrete:
  - `http://127.0.0.1:<QEMU_HOST_PORT>/`
  - `ssh -tt -p <QEMU_SSH_HOST_PORT> 127.0.0.1`

### What was tricky to build

- The tricky part was middleware ordering. Wish middleware executes in a way that makes “last in the list” behavior matter. If `activeterm` is not positioned correctly, a non-PTY client can see app output before being rejected. Reordering the chain fixed that policy leak.

### What warrants a second pair of eyes

- The SSH service still runs with `authMode: "none"`. That is fine for this demo ticket, but it is the next obvious security hardening boundary once persistent storage is available for stable keys and `authorized_keys`.

### What should be done in the future

- Refresh the ticket bundle, then open the follow-up work for persistent storage and real SSH authentication material.

### Code review instructions

- Start with:
  - [Makefile](/home/manuel/code/wesen/2026-03-08--qemu-go-init/Makefile)
  - [scripts/qemu-smoke.sh](/home/manuel/code/wesen/2026-03-08--qemu-go-init/scripts/qemu-smoke.sh)
  - [internal/sshapp/server.go](/home/manuel/code/wesen/2026-03-08--qemu-go-init/internal/sshapp/server.go)
- Re-run:

```bash
bash -n scripts/qemu-smoke.sh
go test ./... -count=1
make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 QEMU_SSH_HOST_PORT=10022
```

### Technical details

Commands used:

```bash
bash -n scripts/qemu-smoke.sh
go test ./... -count=1
make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080 QEMU_SSH_HOST_PORT=10022
```

## Step 8: Validate the ticket metadata and refresh the published bundle

This final step closed the implementation loop. The code was already working in QEMU, but the ticket still needed to accurately reflect the completed task list and to replace the earlier research-only reMarkable bundle with the implemented version. That distinction matters because this ticket started as research and ended as working code; the published artifact needed to match the new reality.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Finish the ticket end to end, including metadata validation, diary/task updates, and publishing the implemented state.

**Inferred user intent:** Leave behind a complete, reviewable implementation trail instead of stopping at code changes.

### What I did

- Updated the current task list and status text in the ticket index.
- Ran `docmgr doctor --ticket QEMU-GO-INIT-004 --stale-after 30`.
- Performed a dry-run bundle upload, then force-uploaded the refreshed bundle because the research-only bundle already existed remotely.
- Verified the remote listing.

### Why

- The ticket history is only useful if the final published bundle matches the code that now exists in the repo.

### What worked

- `docmgr doctor --ticket QEMU-GO-INIT-004 --stale-after 30`
- `remarquee upload bundle --dry-run ...`
- `remarquee upload bundle --force ...`
- `remarquee cloud ls /ai/2026/03/08/QEMU-GO-INIT-004 --long --non-interactive`

### What didn't work

- The first non-force upload was skipped because the earlier bundle already existed:

```text
SKIP: QEMU-GO-INIT-004 bundle already exists in /ai/2026/03/08/QEMU-GO-INIT-004 (use --force to overwrite)
```

- Re-running with `--force` resolved that cleanly.

### What I learned

- ReMarkable publication needs to be treated as part of the ticket lifecycle, not as an afterthought, especially when a ticket changes from “research” to “implemented”.

### What was tricky to build

- The only tricky part was making sure the published artifact reflected the implemented state rather than the earlier research snapshot. That required an explicit overwrite instead of assuming upload would replace the prior bundle automatically.

### What warrants a second pair of eyes

- The implementation is complete for the ticket scope, but the auth model remains intentionally permissive until the persistence ticket lands.

### What should be done in the future

- Use [QEMU-GO-INIT-005](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/index.md) as the next ticket for durable host keys and app state.

### Code review instructions

- Review:
  - [tasks.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/tasks.md)
  - [index.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/index.md)
  - [changelog.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/changelog.md)

### Technical details

Commands used:

```bash
docmgr doctor --ticket QEMU-GO-INIT-004 --stale-after 30
remarquee upload bundle --dry-run ...
remarquee upload bundle --force ...
remarquee cloud ls /ai/2026/03/08/QEMU-GO-INIT-004 --long --non-interactive
```

## Step 3: Validate the ticket and publish the research bundle

This step closed the research loop by validating the ticket metadata, fixing the one vocabulary issue `docmgr doctor` found, and uploading the finished bundle to reMarkable. At this point the ticket had everything the user asked for: a new workspace, a detailed design doc, an implementation guide, a local experiment, and a diary that captured the useful friction instead of hiding it.

The most important part of this step was keeping the bookkeeping as rigorous as the research. It is easy to stop once the design doc exists. It is more useful to leave behind a ticket that is searchable, validated, and already available off-repo.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Finish the research ticket end to end, including the metadata cleanup and final publication steps.

**Inferred user intent:** Produce a complete research artifact, not just a local markdown draft.

**Commit (code):** acabe28ddd7c2cdcbc794263a286f69fb8702839 — "Add Wish SSH research ticket and probe"

### What I did

- Ran `docmgr doctor --ticket QEMU-GO-INIT-004 --stale-after 30`.
- Added the missing `ssh` vocabulary topic with:

```bash
docmgr vocab add --category topics --slug ssh --description "Secure Shell protocols, servers, clients, and SSH-hosted applications"
```

- Re-ran `docmgr doctor` successfully.
- Ran a dry-run and then a real reMarkable bundle upload.
- Verified the remote listing.

### Why

- A research ticket that fails `docmgr doctor` or never gets published is incomplete operationally, even if the document text itself is good.

### What worked

- `docmgr doctor` passed cleanly after the vocabulary fix.
- The upload succeeded:

```text
OK: uploaded QEMU-GO-INIT-004 bundle.pdf -> /ai/2026/03/08/QEMU-GO-INIT-004
```

- Remote verification succeeded:

```text
[f]	QEMU-GO-INIT-004 bundle
```

### What didn't work

- The first `docmgr doctor` run reported:

```text
[WARNING] unknown_topics — unknown topics: [ssh]
```

- That was a bookkeeping issue, not a content issue, and it was fixed by adding the topic to the vocabulary.

### What I learned

- New ticket topic vocabulary is easy to overlook when the technical work itself is moving quickly.
- The dry-run upload continues to be worth doing even for documentation-only tickets.

### What was tricky to build

- The trickiest part of this step was not technical complexity. It was making sure the ticket state matched the actual work and did not stop at “good enough locally.”

### What warrants a second pair of eyes

- The research conclusions are strong for a phase-1 Wish integration, but the eventual implementation ticket still needs an explicit product choice between “custom SSH app” and “broader shell-like SSH behavior.”

### What should be done in the future

- Convert this research ticket into a concrete implementation ticket once the preferred SSH product shape is chosen.

### Code review instructions

- Review:
  - [tasks.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/tasks.md)
  - [changelog.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/changelog.md)
  - [01-wish-based-ssh-app-architecture-analysis-and-implementation-guide-for-the-go-init-runtime.md](/home/manuel/code/wesen/2026-03-08--qemu-go-init/ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/design-doc/01-wish-based-ssh-app-architecture-analysis-and-implementation-guide-for-the-go-init-runtime.md)
- Validate by rerunning the same `docmgr doctor` and `remarquee` commands.

### Technical details

Commands used:

```bash
docmgr doctor --ticket QEMU-GO-INIT-004 --stale-after 30
docmgr vocab add --category topics --slug ssh --description "Secure Shell protocols, servers, clients, and SSH-hosted applications"
docmgr doctor --ticket QEMU-GO-INIT-004 --stale-after 30
remarquee upload bundle --dry-run ...
remarquee upload bundle ...
remarquee cloud ls /ai/2026/03/08/QEMU-GO-INIT-004 --long --non-interactive
```
