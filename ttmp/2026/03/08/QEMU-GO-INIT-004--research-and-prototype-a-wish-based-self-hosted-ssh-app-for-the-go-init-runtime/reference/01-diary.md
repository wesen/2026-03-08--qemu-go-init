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

**Commit (code):** pending

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

**Commit (code):** pending

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

## Step 3: Validate the ticket and publish the research bundle

This step closed the research loop by validating the ticket metadata, fixing the one vocabulary issue `docmgr doctor` found, and uploading the finished bundle to reMarkable. At this point the ticket had everything the user asked for: a new workspace, a detailed design doc, an implementation guide, a local experiment, and a diary that captured the useful friction instead of hiding it.

The most important part of this step was keeping the bookkeeping as rigorous as the research. It is easy to stop once the design doc exists. It is more useful to leave behind a ticket that is searchable, validated, and already available off-repo.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Finish the research ticket end to end, including the metadata cleanup and final publication steps.

**Inferred user intent:** Produce a complete research artifact, not just a local markdown draft.

**Commit (code):** pending

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
