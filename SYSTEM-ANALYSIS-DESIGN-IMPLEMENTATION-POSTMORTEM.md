# qemu-go-init End-State Guide and Postmortem

This document is the end result of the work done in this repository. It is both a system guide and a postmortem. It explains what the system is, how it is built, how the pieces fit together, what we changed over time, what went well, what went badly, and what a new intern should understand before making changes.

The repo started as a minimal experiment: boot a Linux kernel directly under QEMU, run a single Go binary as `/init`, and serve a webpage. It is now a larger system:

- A custom Go PID 1 runtime boots as the guest init process.
- The guest configures its own networking in userspace.
- The guest serves an embedded web UI and JSON debug endpoints.
- The guest runs a Wish-based SSH server.
- SSH drops the user into a Bubble Tea BBS.
- The BBS stores messages in SQLite on shared persistent state.
- The BBS also contains a JavaScript REPL backed by `go-go-goja` and Bobatea.
- The guest mounts shared state from the host through QEMU `9p`.
- The guest persists chat timelines, turns, and logs in SQLite.
- The guest can talk to external AI providers through Pinocchio.
- The repo now ships a dynamically linked CGO guest runtime together with its runtime loader and libraries inside the initramfs.

If you only need a mental model, start with the next section and the diagrams. If you need to change code, read this document linearly.

## 1. Executive Summary

The system is a tiny Linux appliance built from three main parts:

1. A host-side build pipeline that compiles a Go binary, packages an initramfs, and launches QEMU.
2. A guest-side PID 1 runtime that mounts kernel filesystems, loads a small set of modules, brings up storage and networking, and starts application services.
3. A terminal and web application layer that exposes the guest through HTTP, SSH, Bubble Tea, Wish, Pinocchio, SQLite, and shared host/guest storage.

In practical terms, this repo behaves like a very small custom operating system image where the "userland" is mostly one Go program plus a minimal dynamic runtime.

## 2. Current System at a Glance

### 2.1 Top-Level Flow

```text
Host source tree
  -> go build ./cmd/init
  -> collect ELF runtime dependencies
  -> build initramfs.cpio.gz
  -> qemu-system-x86_64 boots kernel + initramfs
  -> kernel execs /init
  -> Go PID 1 mounts proc/sys/dev
  -> Go PID 1 mounts persistent ext4 and shared 9p state
  -> Go PID 1 configures network with DHCP in userspace
  -> Go PID 1 starts HTTP + SSH services
  -> SSH opens Bubble Tea BBS / JS REPL / AI chat
```

### 2.2 Major User-Facing Surfaces

- Web status UI:
  - [`internal/webui/site.go`](internal/webui/site.go)
- SSH server:
  - [`internal/sshapp/server.go`](internal/sshapp/server.go)
- Bubble Tea BBS:
  - [`internal/bbsapp/model.go`](internal/bbsapp/model.go)
- Wish-to-BBS glue:
  - [`internal/sshbbs/middleware.go`](internal/sshbbs/middleware.go)
- JavaScript REPL:
  - [`internal/jsrepl/surface.go`](internal/jsrepl/surface.go)
- AI chat:
  - [`internal/aichat/surface.go`](internal/aichat/surface.go)

### 2.3 Persistence Surfaces

- BBS posts:
  - [`internal/bbsstore/store.go`](internal/bbsstore/store.go)
- AI timeline and turn persistence:
  - [`internal/aichat/persistence.go`](internal/aichat/persistence.go)
- Application log storage:
  - [`internal/logstore/store.go`](internal/logstore/store.go)
- Host-side QEMU log import:
  - [`cmd/importqemulogs/main.go`](cmd/importqemulogs/main.go)

## 3. Why This System Exists

The original goal was intentionally strange but useful:

- Avoid a normal distro userland.
- Prove that Go can act as PID 1.
- Boot directly under QEMU.
- Serve a webpage from the guest.
- Grow the system incrementally without abandoning the single-binary spirit.

That goal stayed constant, but the implementation matured:

- Networking moved from "hope the kernel handles `ip=dhcp`" to a real userspace DHCP client.
- Entropy moved from "hope `/dev/random` works" to explicit `virtio-rng` support.
- Persistence moved from RAM-only initramfs storage to guest disk and shared state.
- SSH moved from "it boots" to a real Wish-driven application surface.
- Chat moved from transient UI state to persisted timelines and turns.
- The JS REPL moved from a minimal evaluator to a tree-sitter-assisted REPL path after CGO support was added.

## 4. Repository Map

### 4.1 Commands

- [`cmd/init/main.go`](cmd/init/main.go)
  - The guest PID 1 runtime.
  - Orchestrates boot, storage, networking, entropy, HTTP, SSH, BBS, JS REPL, AI chat, and logging.
- [`cmd/mkinitramfs/main.go`](cmd/mkinitramfs/main.go)
  - Builds the initramfs archive.
  - Stages `/init`, runtime loader/libs, CA bundle, and selected kernel modules.
- [`cmd/bbs/main.go`](cmd/bbs/main.go)
  - Runs the BBS natively on the host against the shared SQLite state.
- [`cmd/importqemulogs/main.go`](cmd/importqemulogs/main.go)
  - Imports host-captured QEMU serial logs into SQLite.

### 4.2 Core Internal Packages

- [`internal/boot/boot.go`](internal/boot/boot.go)
  - Early mount helpers and PID 1 boot setup.
- [`internal/storage/storage.go`](internal/storage/storage.go)
  - Mounts the persistent guest disk.
- [`internal/sharedstate/sharedstate.go`](internal/sharedstate/sharedstate.go)
  - Mounts the shared `9p` host directory.
- [`internal/networking/network.go`](internal/networking/network.go)
  - NIC discovery, DHCP, IP setup, routing, DNS.
- [`internal/kmod/kmod.go`](internal/kmod/kmod.go)
  - Kernel module loading from the initramfs.
- [`internal/entropy/entropy.go`](internal/entropy/entropy.go)
  - Runtime entropy observation and status reporting.
- [`internal/webui/site.go`](internal/webui/site.go)
  - HTTP routes and debug endpoints.
- [`internal/sshapp/server.go`](internal/sshapp/server.go)
  - Wish server setup and host-key management.
- [`internal/sshbbs/middleware.go`](internal/sshbbs/middleware.go)
  - SSH session -> BBS session adapter.

### 4.3 App Layer

- [`internal/bbsapp/model.go`](internal/bbsapp/model.go)
  - Main terminal UI with browse, compose, JS REPL, and chat modes.
- [`internal/jsrepl/surface.go`](internal/jsrepl/surface.go)
  - Bobatea REPL surface backed by `go-go-goja`.
- [`internal/aichat/surface.go`](internal/aichat/surface.go)
  - Pinocchio-backed AI chat surface.
- [`internal/aichat/debug.go`](internal/aichat/debug.go)
  - Runtime inspection and HTTPS probe support.

## 5. Host Build Pipeline

The build pipeline is driven by [`Makefile`](Makefile).

### 5.1 Important Targets

- `make build`
  - Builds `build/init`.
- `make initramfs`
  - Builds `build/initramfs.cpio.gz`.
- `make run`
  - Boots QEMU with HTTP and SSH host forwards.
- `make smoke`
  - Runs end-to-end verification.
- `make import-qemu-log`
  - Loads QEMU host-side logs into SQLite.

### 5.2 Why the Build Changed Over Time

At first the system was pure-Go and static. That worked until richer upstream packages required CGO-backed dependencies, especially the SQLite and tree-sitter paths we wanted to reuse.

That changed the build model:

- Before:
  - static `CGO_ENABLED=0` `/init`
- After:
  - CGO-enabled `/init`
  - dynamic loader and runtime libs copied into initramfs
  - guest still feels "single appliance", but it is no longer literally one ELF file

### 5.3 Current Build Pseudocode

```text
function build_guest():
  compile cmd/init to build/init with CGO enabled
  inspect build/init ELF dependencies
  write build/init.runtime-file-maps.txt
  run cmd/mkinitramfs with:
    - /init mapping
    - runtime loader/libs mappings
    - CA bundle mapping
    - virtio-rng module mapping
    - 9p-related module mappings
  emit build/initramfs.cpio.gz
```

### 5.4 Why `collect-elf-runtime.sh` Exists

Once `/init` became dynamically linked, the kernel could not start it unless the initramfs already contained:

- the dynamic loader
- `libc`
- any other needed shared objects

The host-side helper:

- [`scripts/collect-elf-runtime.sh`](scripts/collect-elf-runtime.sh)

collects those dependencies ahead of time and hands them to the initramfs builder.

This was one of the major architectural turning points in the project.

## 6. Guest Boot Sequence

### 6.1 Sequence

```text
kernel
  -> unpack initramfs
  -> exec /init
  -> Go PID 1 starts
  -> mount proc, sysfs, devtmpfs
  -> load optional kernel modules
  -> mount ext4 persistent disk
  -> mount 9p shared state
  -> bring up network
  -> configure DNS
  -> start HTTP server
  -> start Wish SSH server
  -> wait as PID 1 and reap children
```

### 6.2 Why PID 1 is Special

PID 1 in Linux is not a normal process.

It must handle:

- mounting key kernel filesystems
- child reaping
- signal behavior that differs from normal processes
- system-wide failures with no supervisor above it

That is why the boot code is separated into:

- [`internal/boot/boot.go`](internal/boot/boot.go)

and why [`cmd/init/main.go`](cmd/init/main.go) is more orchestration-heavy than a normal Go `main`.

## 7. Networking: What We Tried and What Failed

Networking was one of the biggest debugging arcs in the repo.

### 7.1 Original Assumption

The initial assumption was:

- QEMU `user` networking provides DHCP
- if the kernel sees `ip=dhcp`, the guest will come up

That was only half true.

QEMU did provide DHCP, but the host kernel we were booting had:

- built-in virtio-net support
- no kernel IP autoconfiguration support

So:

- the NIC driver existed
- `ip=dhcp` was ignored
- the guest never configured an IP

### 7.2 The Fix

We moved DHCP into userspace:

- [`internal/networking/network.go`](internal/networking/network.go)

Key libraries:

- `github.com/insomniacslk/dhcp/dhcpv4/nclient4`
- `github.com/vishvananda/netlink`

Then we discovered an early-boot entropy interaction that caused the higher-level DHCP helper to stall. The final fix was to drive the DHCP Discover/Offer/Request/Ack exchange more explicitly and deterministically.

### 7.3 Networking Lessons

What worked:

- QEMU `-nic user` plus host forwarding
- userspace DHCP
- netlink-based address and route setup

What did not work:

- assuming a generic distro kernel honors `ip=dhcp`
- assuming "virtio-net built in" means "networking will just work"

### 7.4 Current Network Model

```text
QEMU user-mode NAT + DHCP
  -> guest virtio-net device
  -> userspace DHCP in Go
  -> netlink configures address/route
  -> host reaches guest via hostfwd
```

## 8. Entropy: What We Learned

Entropy looked like a small detail and turned out to matter early.

### 8.1 The Problem

Some code paths indirectly relied on strong randomness earlier than expected. That is dangerous in early boot because:

- entropy pools may be weak or uninitialized
- requests can block
- failures can be misleading

### 8.2 The Fix

We added:

- QEMU `virtio-rng`
- guest runtime support to detect and expose entropy state
- module support for `virtio_rng` in the initramfs when needed

Relevant files:

- [`internal/entropy/entropy.go`](internal/entropy/entropy.go)
- [`internal/kmod/kmod.go`](internal/kmod/kmod.go)
- [`cmd/mkinitramfs/main.go`](cmd/mkinitramfs/main.go)

### 8.3 Entropy Lessons

Good:

- explicitly wiring entropy is cheap and worth it

Bad:

- hidden randomness dependencies are easy to miss
- "it works on my host" can conceal early-boot stalls

## 9. Persistence and Shared State

Persistence evolved in two layers because the system needed two different storage stories.

### 9.1 Guest Local Persistent Disk

The guest has an ext4 disk image mounted through:

- [`internal/storage/storage.go`](internal/storage/storage.go)

This is used for guest-owned durable state such as:

- SSH host keys
- guest-local runtime state

### 9.2 Host/Guest Shared State

For the BBS and host-native app experience, a host directory is shared into the guest over `9p`:

- [`internal/sharedstate/sharedstate.go`](internal/sharedstate/sharedstate.go)

This made the important product goal possible:

- run the BBS natively on the host
- run the same BBS over SSH in the guest
- point both at the same shared SQLite-backed state directory

### 9.3 Why We Chose Shared State Instead of Raw ext4 for Everything

A raw ext4 image is fine for guest persistence across reboot, but it is awkward for direct host access because the host cannot simply open files inside the image without mounting it or using special tooling.

Shared host/guest state via `9p` made development and product behavior much easier.

## 10. SSH Application Layer

### 10.1 Wish Server

The SSH server is built around:

- [`internal/sshapp/server.go`](internal/sshapp/server.go)

and adapted into the terminal application with:

- [`internal/sshbbs/middleware.go`](internal/sshbbs/middleware.go)

Wish let us keep the SSH server inside Go without requiring OpenSSH userland.

### 10.2 Early Constraints

At first:

- no persistence for host keys
- no persisted `authorized_keys`
- demo auth mode

That was enough to prove the surface, but not enough for a realistic service.

### 10.3 Improvements

We added:

- persistent host-key storage
- better host-key write validation
- more explicit runtime reporting

## 11. The Bubble Tea BBS

### 11.1 Main Modes

The main UI is in:

- [`internal/bbsapp/model.go`](internal/bbsapp/model.go)

The user can switch between:

- browse mode
- compose mode
- JS REPL mode
- AI chat mode

### 11.2 Data Model

The BBS stores posts in SQLite through:

- [`internal/bbsstore/store.go`](internal/bbsstore/store.go)

That store is intentionally simple and stable. It is the foundation that both:

- the host-native app
- the guest SSH app

use for shared board content.

### 11.3 Why the Host-Native BBS Matters

The host-native app in:

- [`cmd/bbs/main.go`](cmd/bbs/main.go)

is not a side demo. It is a useful architectural proof:

- the UI logic is not tied to the guest
- the persistence path is not tied to the guest
- the guest is acting as a deployment target, not the only runtime

## 12. JavaScript REPL

### 12.1 Early State

The first JS REPL surface was intentionally minimal:

- evaluate JavaScript
- expose `bbs.*`
- no AST completion
- no contextual help

That was a reasonable first step while the guest was still strictly `CGO_ENABLED=0`.

### 12.2 Current State

The REPL now uses the richer upstream adapter path:

- [`internal/jsrepl/surface.go`](internal/jsrepl/surface.go)

and integrates with:

- `go-go-goja`
- Bobatea REPL widgets
- tree-sitter-backed completion/help behavior

The `bbs` globals are installed into the JS runtime, so the REPL can:

- list posts
- create posts
- inspect shared state

### 12.3 Current REPL UX

From the REPL:

- `Tab` triggers completion
- `Ctrl+T` toggles focus to the timeline
- `Alt+H` opens contextual docs
- `Ctrl+P` opens the command palette

We explicitly pinned those bindings in [`internal/jsrepl/surface.go`](internal/jsrepl/surface.go) after observing that `Tab` could otherwise fall through to focus toggling in stale or misconfigured builds.

### 12.4 Important Limitation

The system now has AST-driven completion and help, but not fully colorized inline syntax highlighting in the input field itself. The current Bobatea REPL experience is:

- completion popup
- help bar
- help drawer
- command palette

That is still a good improvement, but it is not a full IDE-like editor surface.

## 13. AI Chat

### 13.1 Architecture

The AI chat surface lives in:

- [`internal/aichat/surface.go`](internal/aichat/surface.go)

It uses Pinocchio and the broader upstream model stack for provider access and conversation state.

### 13.2 How the Chat Knows About BBS Posts

The chat does not query BBS posts dynamically on every turn. Instead:

- it reads recent posts from the BBS store
- formats them into a seed turn
- provides that seed context when the chat surface is created

That means the model is aware of recent board context, but it is snapshot-based rather than live query-based.

### 13.3 Debugging the Chat Stack

The chat path required several rounds of debugging:

- config and profile path resolution
- missing CA roots in the guest
- chat backend failures not surfacing as UI-visible errors
- logs overwhelming the terminal session

Useful files:

- [`internal/aichat/debug.go`](internal/aichat/debug.go)
- [`internal/webui/site.go`](internal/webui/site.go)
- [`internal/zlog/config.go`](internal/zlog/config.go)

### 13.4 Current Debug Endpoints

The web UI exposes endpoints for:

- runtime profile resolution
- HTTPS probe verification
- debug inspection of chat runtime inputs

These were essential to separate:

- transport failures
- TLS trust failures
- missing config
- UI error propagation problems

## 14. SQLite Persistence

### 14.1 What Is Stored

SQLite now stores:

- BBS posts
- AI turns
- AI timeline state
- application logs
- imported host-side QEMU logs

### 14.2 Why We Switched to Upstream CGO SQLite Stores

At one point we maintained CGO-free SQLite usage through `modernc.org/sqlite`, but the desire to reuse upstream Pinocchio timeline and turn stores pushed the system toward:

- CGO-enabled SQLite
- upstream store reuse

This was a pragmatic trade:

- less custom persistence glue
- more consistent data model with upstream tools
- more complex runtime packaging

### 14.3 Consequence

This was the point where the repo stopped being "one pure static binary" and became:

- a Go-centric tiny appliance
- with a dynamically linked runtime staged into the initramfs

That trade was reasonable. It increased complexity, but it bought reuse and feature velocity.

## 15. Logging Strategy

### 15.1 There Are Two Different Kinds of Logs

1. Guest application logs
2. Host-side QEMU serial logs

These are not the same thing.

### 15.2 Guest Logs

Guest logs are produced by the Go runtime and app stack and can be written directly into guest-available storage.

### 15.3 Host QEMU Logs

QEMU serial logs originate on the host side, outside the guest. They must be imported later if we want them in SQLite.

That is why the repo has:

- [`cmd/importqemulogs/main.go`](cmd/importqemulogs/main.go)

### 15.4 Lesson

Do not confuse "logs visible in the terminal while QEMU is running" with "logs available inside the guest." They live on opposite sides of the VM boundary.

## 16. Detailed Milestone Timeline

This is the high-level chronological progression of the system.

### 16.1 Phase 1: Single Go `/init` + Web Page

Goal:

- prove the guest can boot and serve HTTP

Key result:

- yes, a Go PID 1 serving an embedded page is viable

### 16.2 Phase 2: Userspace Networking

Problem:

- host kernel ignored `ip=dhcp`

Fix:

- Go DHCP client + netlink configuration

### 16.3 Phase 3: Early-Boot Entropy Support

Problem:

- hidden runtime behavior stalled or risked weak randomness

Fix:

- QEMU `virtio-rng`, module support, runtime diagnostics

### 16.4 Phase 4: Wish SSH Surface

Goal:

- provide a fully self-hosted SSH application surface

Result:

- Wish server launched directly from the guest runtime

### 16.5 Phase 5: Persistent Storage

Goal:

- persist host keys and app state across reboot

Result:

- guest disk + mount lifecycle

### 16.6 Phase 6: Shared-State BBS

Goal:

- one BBS app shared across host and guest

Result:

- host/guest shared directory with SQLite-backed BBS

### 16.7 Phase 7: JS REPL

Goal:

- manipulate the BBS from inside the SSH session

Result:

- first minimal REPL, later upgraded to tree-sitter-backed completion/help

### 16.8 Phase 8: AI Chat

Goal:

- integrate an LLM chat surface into the SSH appliance

Result:

- working chat with persisted state and debug endpoints

### 16.9 Phase 9: CGO Runtime and Upstream SQLite Store Reuse

Goal:

- reuse richer upstream persistence rather than duplicating it locally

Result:

- dynamic runtime packaging inside initramfs

## 17. What Was Good

### 17.1 Good Architectural Choices

- Keeping host build logic explicit in [`Makefile`](Makefile)
- Treating the guest like a product appliance rather than a random shell environment
- Moving networking into userspace rather than depending on kernel luck
- Separating guest-local persistence from host/guest shared persistence
- Keeping the BBS runnable natively on the host
- Reusing upstream libraries where the integration cost was justified
- Adding debug endpoints instead of only reading serial logs

### 17.2 Good Process Choices

- Writing detailed ticket docs during the project
- Keeping diaries and playbooks during debugging
- Verifying assumptions with packet capture and runtime inspection
- Testing both host-native and guest-SSH paths

## 18. What Was Bad

### 18.1 Bad Assumptions

- assuming host kernels would honor `ip=dhcp`
- assuming `virtio-net` built-in support was sufficient
- assuming entropy would not matter early
- assuming TLS roots would "just exist" in the guest
- assuming dynamic chat failures would be obvious from the TUI alone
- assuming a source change had been deployed to a running VM without rebuilding initramfs

### 18.2 Bad UX Moments

- provider failures sometimes only surfaced as logs at first
- debug logs occasionally polluted terminal views
- REPL capability changes were easy to miss if the wrong VM instance was still running
- stale QEMU sessions caused confusing "nothing changed" reports

### 18.3 Bad Structural Tension

The biggest architectural tension in the repo is this:

- the project wants to stay tiny and self-contained
- advanced dependencies pull it toward a richer runtime surface

There is no perfect answer. The current result is reasonable, but it is not as conceptually pure as the original static-single-binary goal.

## 19. Biggest Lessons

### 19.1 Lesson: Boot Systems Need Explicitness

In normal applications, many assumptions are hidden by the host OS. In this repo they are not.

You must be explicit about:

- mounts
- entropy
- networking
- certificates
- dynamic loader files
- module availability
- storage

### 19.2 Lesson: Reuse Is Not Free

Reusing upstream SQLite persistence and tree-sitter support saved feature work, but increased:

- build complexity
- runtime packaging complexity
- deployment assumptions

This was still the right trade, but it was not free.

### 19.3 Lesson: Host and Guest Need Separate Mental Models

A lot of confusion disappears if you separate:

- host filesystem
- guest filesystem
- host logs
- guest logs
- host build artifacts
- live guest deployment state

This repo crosses that boundary constantly.

## 20. Operational Guide for a New Intern

### 20.1 First Commands to Know

```bash
make test
make initramfs KERNEL_IMAGE=qemu-vmlinuz
make run KERNEL_IMAGE=qemu-vmlinuz
make smoke KERNEL_IMAGE=qemu-vmlinuz
go run ./cmd/bbs -state-root build/shared-state/bbs
```

### 20.2 If the Guest Does Not Change

Check these in order:

1. Did you rebuild `build/init`?
2. Did you rebuild `build/initramfs.cpio.gz`?
3. Did you restart QEMU?
4. Are you SSHing into the correct forwarded port?
5. Is the running VM using a different `QEMU_DATA_IMAGE` or shared-state path than you expect?

### 20.3 If HTTPS Fails

Check:

- guest CA bundle staging in [`cmd/mkinitramfs/main.go`](cmd/mkinitramfs/main.go)
- runtime debug endpoints in [`internal/aichat/debug.go`](internal/aichat/debug.go)
- provider config copy into shared state from [`Makefile`](Makefile)

### 20.4 If REPL Completion Seems Broken

Check:

- that you are on a rebuilt VM
- that `Tab` is showing the completion popup
- that `Ctrl+T` is the focus toggle
- that `Alt+H` opens the docs drawer

Relevant file:

- [`internal/jsrepl/surface.go`](internal/jsrepl/surface.go)

## 21. Change Procedure Recommendations

When changing this system, use this order:

1. Update the host-native path first when possible.
2. Add targeted unit tests for the changed package.
3. Rebuild `build/init` and `build/initramfs.cpio.gz`.
4. Boot a fresh QEMU instance on clean ports.
5. Verify both HTTP and SSH paths.
6. If the change touches chat or network, inspect the debug endpoints.
7. If the change touches storage, verify persistence across reboot.

## 22. Open Issues and Future Work

- Public-key auth is still the next obvious SSH hardening step.
- The chat surface does not currently expose real tools.
- The REPL still lacks full inline syntax highlighting.
- The command palette is useful but not yet application-specific.
- The host and guest log flows are now better, but still operationally separate.
- The guest runtime is more complex than the original static-only design and could benefit from another cleanup pass.

## 23. Final Assessment

This project succeeded at the important thing: it demonstrated that a Go-first tiny guest appliance can grow far beyond a toy demo without collapsing into a full distro image.

That said, the repo also proved an equally important counterpoint:

- the deeper you go into terminals, SQLite reuse, tree-sitter, and provider integrations
- the more you must acknowledge runtime realities such as libc, CA roots, module staging, and storage boundaries

The current design is good because it is honest about that complexity. It keeps the system small and understandable, but it no longer pretends the system is simpler than it is.

For a new intern, the right mindset is:

- treat the repo as a tiny operating system plus applications
- always think about host versus guest
- always think about build artifact versus running VM
- verify behavior end to end, not just at compile time

If you keep those four rules in mind, the rest of the system will make sense.
