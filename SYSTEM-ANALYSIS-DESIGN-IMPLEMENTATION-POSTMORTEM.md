# qemu-go-init
## End-State Analysis, Design, Implementation Guide, and Postmortem

This is the document I would hand to a new intern on day one.

It has four jobs:

1. Explain what this repository is.
2. Explain how the system actually works, from build to boot to runtime behavior.
3. Explain how we got here, including the ticket-by-ticket evolution.
4. Explain what went right, what went wrong, and what you should be careful about next.

This repository started as a deliberately constrained systems experiment:

- boot a Linux kernel under QEMU
- provide a custom initramfs
- run a Go binary as `/init`
- avoid a traditional distro userland
- serve a webpage

It ended up as something much richer:

- a Go PID 1 runtime
- host-side build tooling
- userspace DHCP and netlink configuration
- entropy plumbing
- a Wish SSH server
- a Bubble Tea BBS
- a shared-state SQLite setup that works from both host and guest
- a JavaScript REPL
- an AI chat surface with Pinocchio
- persisted timelines, turns, and logs
- a CGO-enabled guest runtime with dynamic loader and runtime libs staged into the initramfs

This document is intentionally long. The goal is not brevity. The goal is to make the system legible.

---

## 1. Reading Guide

If you are new to the repo, read in this order:

1. “System in One Page”
2. “Vocabulary”
3. “Architecture Walkthrough”
4. “Boot Sequence”
5. “Storage and Data Flow”
6. “Timeline of Work”
7. “Postmortem”

If you need to make changes immediately, jump to:

- “File-by-File System Tour”
- “Biggest Technical Lessons”
- “Recommendations for a New Intern”

If you want the detailed historical record, the ticket docs live under [`ttmp/`](ttmp/). The most useful ones are linked throughout this guide.

---

## 2. System in One Page

The shortest accurate description of the current system is:

> `qemu-go-init` is a tiny Go-driven guest appliance that boots directly under QEMU, runs a custom Go PID 1 process, mounts both persistent guest storage and host-shared state, serves a debug/status web UI, runs an SSH server with a Bubble Tea application, stores state in SQLite, and exposes both a JavaScript REPL and an AI chat surface inside the SSH session.

That sentence is dense, so here is the same idea as a diagram:

```text
                       HOST MACHINE
┌─────────────────────────────────────────────────────────────────────┐
│ repo checkout                                                      │
│   ├── Makefile                                                     │
│   ├── cmd/init                                                     │
│   ├── cmd/mkinitramfs                                              │
│   ├── build/init                                                   │
│   ├── build/initramfs.cpio.gz                                      │
│   ├── build/data.img                guest-local persistent storage  │
│   └── build/shared-state/           host/guest shared state        │
│                                                                     │
│ qemu-system-x86_64                                                  │
│   ├── kernel                                                        │
│   ├── initramfs                                                     │
│   ├── user-mode NAT + DHCP                                          │
│   ├── hostfwd :18086 -> guest :8080                                 │
│   ├── hostfwd :10028 -> guest :2222                                 │
│   └── 9p shared dir -> guest /var/lib/go-init/shared                │
└─────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
                           QEMU GUEST
┌─────────────────────────────────────────────────────────────────────┐
│ kernel                                                             │
│   -> exec /init                                                    │
│                                                                     │
│ /init = Go PID 1                                                   │
│   ├── mount proc/sys/dev                                           │
│   ├── load small module set                                        │
│   ├── mount ext4 persistent disk                                   │
│   ├── mount 9p shared state                                        │
│   ├── configure DHCP + routes + DNS                                │
│   ├── start HTTP web/debug UI                                      │
│   ├── start Wish SSH server                                        │
│   └── host BBS / JS REPL / AI chat                                 │
│                                                                     │
│ services                                                           │
│   ├── HTTP status/debug UI on :8080                                │
│   ├── Wish SSH app on :2222                                        │
│   ├── Bubble Tea BBS                                               │
│   ├── JS REPL with BBS globals                                     │
│   ├── Pinocchio-backed AI chat                                     │
│   └── SQLite stores for posts, turns, timelines, logs              │
└─────────────────────────────────────────────────────────────────────┘
```

The most important architectural fact is this:

> this repo is not just a Go application anymore; it is a tiny buildable operating environment plus a set of applications that run inside that environment.

That framing clears up a lot of confusion later.

---

## 3. Vocabulary

Before you touch the code, keep these words straight.

### Host

The host is your real machine, the one where you:

- run `make`
- run `qemu-system-x86_64`
- edit files in this repository
- can also run the BBS natively with [`cmd/bbs/main.go`](cmd/bbs/main.go)

Examples of host-side paths:

- [`build/init`](build/init)
- [`build/initramfs.cpio.gz`](build/initramfs.cpio.gz)
- [`build/data.img`](build/data.img)
- [`build/shared-state/`](build/shared-state)

### Guest

The guest is the virtual machine booted by QEMU.

Inside the guest, the important paths are:

- `/init`
- `/var/lib/go-init`
- `/var/lib/go-init/shared`
- `/etc/ssl/certs/ca-certificates.crt`

### Initramfs

The initramfs is the compressed archive QEMU passes to the kernel at boot.

It contains:

- the `/init` binary
- the dynamic loader and shared libraries needed by `/init`
- selected kernel modules
- the CA bundle
- a minimal early userspace layout

### PID 1

PID 1 is the first userspace process started by the kernel.

In this repo, PID 1 is implemented in:

- [`cmd/init/main.go`](cmd/init/main.go)

This matters because PID 1 has real Linux-specific responsibilities:

- mount key filesystems
- reap child processes
- stay alive as the top-level process
- react correctly to failures that a normal app would delegate to the OS

### Persistent Storage vs Shared State

These are different and the distinction matters.

Persistent guest storage:

- guest-local
- ext4 image
- used for things like persisted SSH host keys

Shared state:

- a host directory shared into the guest over `9p`
- intended to be readable and writable by both the host-native app and the guest app
- currently used for the BBS SQLite state and Pinocchio config/profile material

### Source Change vs Deployed Guest

One of the most common mistakes during this project was assuming:

- "I changed source code, therefore the running VM is now using it"

That is false.

The full chain is:

```text
edit source
  -> rebuild build/init
  -> rebuild build/initramfs.cpio.gz
  -> restart QEMU
  -> reconnect to the new guest
```

If any of those steps are skipped, you are not testing what you think you are testing.

---

## 4. What We Built, in Plain English

This project evolved in layers. Each new layer solved a concrete problem but also changed the nature of the repo.

### Layer 1: Single Go `/init` and a Web Page

The first version was conceptually simple:

- compile a Go binary
- call it `/init`
- put it in an initramfs
- boot it under QEMU
- serve an embedded HTTP page

That work is documented in:

- [`ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/design-doc/01-single-binary-go-init-architecture-and-implementation-guide.md`](ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/design-doc/01-single-binary-go-init-architecture-and-implementation-guide.md)
- [`ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/reference/01-diary.md`](ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/reference/01-diary.md)

The important thing that version proved was not just "HTTP works". It proved:

- Go can be PID 1 for this kind of appliance
- a custom initramfs pipeline is manageable
- QEMU is a workable deployment target for the experiment

### Layer 2: Real Networking

The first networking idea was to rely on kernel `ip=dhcp` support, but that failed on the host kernel we were booting.

What we learned:

- QEMU user-mode networking did provide DHCP
- the guest NIC driver could be present
- the kernel could still ignore `ip=dhcp`

The fix was to move network bring-up into Go userspace:

- raw DHCP exchange
- netlink address assignment
- default route setup
- DNS setup

Key docs:

- [`ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/design-doc/01-userspace-dhcp-architecture-and-implementation-guide-for-the-go-init-runtime.md`](ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/design-doc/01-userspace-dhcp-architecture-and-implementation-guide-for-the-go-init-runtime.md)
- [`ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/playbook/01-dhcp-packet-capture-and-inspection-playbook.md`](ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/playbook/01-dhcp-packet-capture-and-inspection-playbook.md)
- [`ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/reference/03-postmortem-early-boot-dhcp-entropy-stall-and-recovery.md`](ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/reference/03-postmortem-early-boot-dhcp-entropy-stall-and-recovery.md)

### Layer 3: Entropy

Once networking and early boot were more serious, entropy stopped being theoretical.

We added:

- QEMU `virtio-rng`
- guest-side entropy diagnostics
- module staging and loading

Key docs:

- [`ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/design-doc/01-early-boot-entropy-support-architecture-and-implementation-guide-for-the-qemu-go-init-runtime.md`](ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/design-doc/01-early-boot-entropy-support-architecture-and-implementation-guide-for-the-qemu-go-init-runtime.md)
- [`ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/design-doc/02-full-system-architecture-usage-and-extension-guide-for-qemu-go-init.md`](ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/design-doc/02-full-system-architecture-usage-and-extension-guide-for-qemu-go-init.md)

### Layer 4: SSH

The next step was turning the appliance into something interactive over SSH.

We used Wish because it let the guest self-host SSH without pulling in OpenSSH userland.

Key docs:

- [`ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/design-doc/01-wish-based-ssh-app-architecture-analysis-and-implementation-guide-for-the-go-init-runtime.md`](ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/design-doc/01-wish-based-ssh-app-architecture-analysis-and-implementation-guide-for-the-go-init-runtime.md)

### Layer 5: Persistence

Once SSH existed, ephemeral state became annoying immediately:

- host keys changing every boot
- config disappearing
- nothing durable

That led to the guest-local persistent disk:

- [`ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/design-doc/01-persistent-guest-storage-architecture-and-implementation-plan-for-qemu-go-init.md`](ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/design-doc/01-persistent-guest-storage-architecture-and-implementation-plan-for-qemu-go-init.md)

### Layer 6: Shared-State BBS

Then the project became more interesting.

Instead of keeping the guest as a sealed box, we decided the app state should also be usable directly from the host.

That produced:

- a Bubble Tea BBS
- a shared SQLite store
- a host-native BBS app
- a guest SSH BBS app
- one shared directory used by both

Key docs:

- [`ttmp/2026/03/08/QEMU-GO-INIT-006--design-a-persistent-sqlite-backed-bubble-tea-bbs-for-host-and-ssh-use/design-doc/01-sqlite-backed-bubble-tea-bbs-architecture-analysis-and-implementation-guide-for-host-and-guest-runtimes.md`](ttmp/2026/03/08/QEMU-GO-INIT-006--design-a-persistent-sqlite-backed-bubble-tea-bbs-architecture-analysis-and-implementation-guide-for-host-and-guest-runtimes.md)
- [`ttmp/2026/03/08/QEMU-GO-INIT-006--design-a-persistent-sqlite-backed-bubble-tea-bbs-for-host-and-ssh-use/reference/01-diary.md`](ttmp/2026/03/08/QEMU-GO-INIT-006--design-a-persistent-sqlite-backed-bubble-tea-bbs-for-host-and-guest-runtimes/reference/01-diary.md)

### Layer 7: JavaScript REPL

The BBS then grew a JavaScript REPL:

- first as a minimal evaluator
- later as a richer tree-sitter-backed REPL after the runtime moved to CGO

Key docs:

- [`ttmp/2026/03/08/QEMU-GO-INIT-007--add-a-javascript-repl-to-the-bubble-tea-bbs-using-go-go-goja-and-bobatea/design-doc/01-javascript-repl-architecture-and-implementation-guide-for-the-bubble-tea-bbs.md`](ttmp/2026/03/08/QEMU-GO-INIT-007--add-a-javascript-repl-to-the-bubble-tea-bbs-using-go-go-goja-and-bobatea/design-doc/01-javascript-repl-architecture-and-implementation-guide-for-the-bubble-tea-bbs.md)

### Layer 8: AI Chat Debugging

Once chat existed, we had to make it debuggable:

- runtime inspection
- HTTPS probing
- config/profile debugging
- better error surfacing

Key docs:

- [`ttmp/2026/03/09/QEMU-GO-INIT-008--debug-pinocchio-chat-connectivity-and-expose-profile-registry-inspection-endpoints/reference/01-diary.md`](ttmp/2026/03/09/QEMU-GO-INIT-008--debug-pinocchio-chat-connectivity-and-expose-profile-registry-inspection-endpoints/reference/01-diary.md)

### Layer 9: Upstream SQLite Reuse and CGO Runtime Packaging

The final major shift was deciding to reuse upstream SQLite-backed timeline and turn stores instead of keeping all storage logic local and CGO-free.

That bought us:

- more upstream-compatible persistence
- less local reinvention
- more features

But it also forced:

- dynamic loader staging
- shared library staging
- a more complex initramfs

Key docs:

- [`ttmp/2026/03/09/QEMU-GO-INIT-009--migrate-guest-chat-persistence-and-logging-to-upstream-cgo-backed-sqlite-stores/reference/02-implementation-guide.md`](ttmp/2026/03/09/QEMU-GO-INIT-009--migrate-guest-chat-persistence-and-logging-to-upstream-cgo-backed-sqlite-stores/reference/02-implementation-guide.md)
- [`ttmp/2026/03/09/QEMU-GO-INIT-009--migrate-guest-chat-persistence-and-logging-to-upstream-cgo-backed-sqlite-stores/reference/01-diary.md`](ttmp/2026/03/09/QEMU-GO-INIT-009--migrate-guest-chat-persistence-and-logging-to-upstream-cgo-backed-sqlite-stores/reference/01-diary.md)

That was the moment where the repo stopped being "one static Go file pretending to be an OS" and became "a tiny OS-like runtime built around Go".

That is fine. It is just important to say it honestly.

---

## 5. Architecture Walkthrough

This section walks the system from outside to inside.

### 5.1 Host Build Layer

Relevant files:

- [`Makefile`](Makefile)
- [`cmd/mkinitramfs/main.go`](cmd/mkinitramfs/main.go)
- [`scripts/collect-elf-runtime.sh`](scripts/collect-elf-runtime.sh)

The host layer is responsible for producing the artifacts that the guest can boot.

The rough flow is:

```text
source files
  -> build/init
  -> build/init.runtime-file-maps.txt
  -> build/initramfs.cpio.gz
  -> qemu-system-x86_64 ...
```

What each artifact means:

- `build/init`
  - the guest `/init` binary
- `build/init.runtime-file-maps.txt`
  - mappings of runtime loader/shared object files that the binary needs
- `build/initramfs.cpio.gz`
  - the actual archive passed to the kernel

The most important architectural change here was introducing dynamic runtime staging.

Originally:

- `build/init` was a static Go binary

Now:

- `build/init` is built with `CGO_ENABLED=1`
- the host collects its ELF runtime dependencies
- the initramfs builder stages those files into the archive

That change is why the initramfs builder matters more now than it did in the first ticket.

### 5.2 Guest Runtime Layer

Relevant files:

- [`cmd/init/main.go`](cmd/init/main.go)
- [`internal/boot/boot.go`](internal/boot/boot.go)
- [`internal/kmod/kmod.go`](internal/kmod/kmod.go)
- [`internal/storage/storage.go`](internal/storage/storage.go)
- [`internal/sharedstate/sharedstate.go`](internal/sharedstate/sharedstate.go)
- [`internal/networking/network.go`](internal/networking/network.go)
- [`internal/entropy/entropy.go`](internal/entropy/entropy.go)

This layer is the machine room.

Its job is to make the guest environment usable before any interesting application code runs.

Responsibilities:

- mount `proc`, `sysfs`, `devtmpfs`
- optionally load needed modules
- mount the persistent guest disk
- mount the shared `9p` state directory
- configure networking
- configure DNS
- observe entropy status
- launch application services

### 5.3 Application Layer

Relevant files:

- [`internal/webui/site.go`](internal/webui/site.go)
- [`internal/sshapp/server.go`](internal/sshapp/server.go)
- [`internal/sshbbs/middleware.go`](internal/sshbbs/middleware.go)
- [`internal/bbsapp/model.go`](internal/bbsapp/model.go)
- [`internal/jsrepl/surface.go`](internal/jsrepl/surface.go)
- [`internal/aichat/surface.go`](internal/aichat/surface.go)

This is the part most users actually experience:

- HTTP status page
- SSH terminal UI
- BBS
- JS REPL
- AI chat

One of the strongest outcomes of the project is that this layer is not guest-exclusive anymore. The BBS can also run on the host via:

- [`cmd/bbs/main.go`](cmd/bbs/main.go)

That means the runtime architecture and the app architecture are related, but not identical.

### 5.4 Data and Persistence Layer

Relevant files:

- [`internal/bbsstore/store.go`](internal/bbsstore/store.go)
- [`internal/aichat/persistence.go`](internal/aichat/persistence.go)
- [`internal/logstore/store.go`](internal/logstore/store.go)
- [`cmd/importqemulogs/main.go`](cmd/importqemulogs/main.go)

This layer handles:

- BBS posts
- AI turns
- AI timelines
- app logs
- imported QEMU logs

The repo currently uses SQLite in more than one role:

- simple application state store
- upstream-compatible chat persistence store
- operational/debug log sink

That is powerful, but it also means schema changes are now more important than they were early in the project.

---

## 6. The Boot Path, Step by Step

This section explains exactly what happens from `make run` to a usable guest.

### 6.1 Host-Side Invocation

The relevant `Makefile` path is:

```text
make run
  -> ensure build/init exists
  -> ensure build/initramfs.cpio.gz exists
  -> ensure data image exists
  -> ensure shared-state directory exists
  -> copy Pinocchio config/profile files into shared state
  -> launch qemu-system-x86_64
```

### 6.2 QEMU Launch

The important QEMU ingredients are:

- kernel image
- initramfs
- host forwarding for HTTP and SSH
- user-mode networking
- virtio-rng
- persistent disk image
- `9p` shared directory

Conceptual launch shape:

```text
qemu-system-x86_64
  -kernel <kernel>
  -initrd build/initramfs.cpio.gz
  -append "console=ttyS0 rdinit=/init"
  -drive file=build/data.img,...
  -virtfs local,path=build/shared-state,...
  -nic user,hostfwd=tcp::<host-http>-:8080,hostfwd=tcp::<host-ssh>-:2222
  -object rng-random,...
  -device virtio-rng-pci,...
```

### 6.3 Kernel Boot

The kernel:

- unpacks the initramfs
- finds `/init`
- if `/init` is dynamically linked, runs the loader staged into the initramfs
- maps the required shared libraries
- finally executes the Go runtime

This is the exact reason we had to stage libc and the loader before `/init` starts. The program cannot "load libc later" if the binary itself is dynamically linked.

### 6.4 PID 1 Startup

The Go PID 1 process then performs the early system setup.

In high-level pseudocode:

```text
main():
  mountKernelFilesystems()
  maybeLoadKernelModules()
  mountPersistentDisk()
  mountSharedState()
  maybeActivateEntropySupport()
  configureNetworking()
  startHTTPServer()
  startSSHServer()
  reapChildrenAndWait()
```

### 6.5 Why the Order Matters

The order is not arbitrary.

For example:

- networking before CA roots would not help TLS if the filesystem layout is still incomplete
- starting SSH before host-key paths are mounted would cause persistence bugs
- starting app surfaces before shared state is mounted would point them at the wrong files
- trying to use provider config before the shared `pinocchio/` directory is available would fail

This repo is full of those ordering constraints.

---

## 7. Storage and Data Flow

Storage became one of the defining design topics in the repo.

### 7.1 There Are Three Storage Zones

```text
Zone A: initramfs
  - early boot only
  - unpacked fresh every boot
  - not persistent

Zone B: guest-local persistent disk
  - ext4 image
  - persists across reboot
  - guest-owned state

Zone C: host/guest shared state
  - host directory
  - mounted into guest via 9p
  - used by both host-native and guest apps
```

This distinction solved multiple problems at once.

### 7.2 Why We Needed Both B and C

Guest-local persistent storage was needed for:

- SSH host keys
- guest-owned runtime files

Shared state was needed for:

- host-native BBS using the same DB as the guest BBS
- sharing Pinocchio config/profile material into the guest
- easier operational visibility from the host

If we had tried to put everything inside a raw ext4 guest image, the host-native BBS would have become much more awkward because the host would not have had simple file-level access to the DB.

### 7.3 Current Mount Story

Guest-local disk:

- mounted by [`internal/storage/storage.go`](internal/storage/storage.go)

Shared state:

- mounted by [`internal/sharedstate/sharedstate.go`](internal/sharedstate/sharedstate.go)

### 7.4 Storage Flow Diagram

```text
HOST
  build/data.img --------------------------> guest ext4 mount
  build/shared-state/ ---------------------> guest /var/lib/go-init/shared
  ~/.pinocchio/config.yaml ---- copy -----> build/shared-state/pinocchio/config.yaml
  ~/.config/pinocchio/profiles.yaml -----> build/shared-state/pinocchio/profiles.yaml

GUEST
  /var/lib/go-init/ssh/...                 persisted host keys
  /var/lib/go-init/shared/bbs/...          shared BBS sqlite state
  /var/lib/go-init/shared/pinocchio/...    provider config and profiles
```

### 7.5 SQLite Placement

Current practical split:

- BBS data lives in shared state so both host and guest can see it
- guest-local system state stays on the ext4 disk
- imported logs and app logs can also be pointed at persistent/shared locations depending on the workflow

---

## 8. Networking, in Painful Detail

Networking was the longest debugging thread in the project, and understanding it is worth the time.

### 8.1 The Original Wrong Mental Model

The naive mental model was:

```text
QEMU user networking provides DHCP
  -> kernel sees ip=dhcp
  -> interface comes up
  -> done
```

This failed because:

- QEMU really did provide DHCP
- the guest kernel did have the virtio-net driver
- but the guest kernel did not support the relevant kernel IP autoconfig path

So `ip=dhcp` was silently ineffective for our needs.

### 8.2 The Actual Solution

We moved the network configuration path into the Go runtime itself.

Relevant file:

- [`internal/networking/network.go`](internal/networking/network.go)

Key pieces:

- find the interface
- bring it up
- speak DHCP in userspace
- assign the leased address
- install routes
- write DNS config

### 8.3 Why This Was Better Anyway

This made the system:

- more explicit
- less dependent on host kernel config luck
- easier to debug
- more aligned with the project’s "Go owns the environment" philosophy

### 8.4 Debugging Playbook

When this layer was failing, the best tool turned out to be packet capture:

- see the DHCP Discover/Offer/Request/Ack path
- confirm whether the guest transmitted
- confirm whether QEMU replied
- isolate whether the failure was:
  - no NIC
  - no packet transmission
  - no DHCP server
  - no ACK processing
  - no route/application after lease

The dedicated playbook is still worth reading:

- [`ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/playbook/01-dhcp-packet-capture-and-inspection-playbook.md`](ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/playbook/01-dhcp-packet-capture-and-inspection-playbook.md)

### 8.5 Networking Lessons

Good:

- QEMU `user` networking plus host forwarding was the right default
- userspace DHCP was cleaner than waiting for the perfect kernel config

Bad:

- generic distro kernels are not safe assumptions for early userspace work
- networking bugs are easy to misdiagnose as HTTP, SSH, or app bugs

---

## 9. Entropy and Early Boot Randomness

Entropy looked minor at first, but it touched multiple failure modes.

### 9.1 What Went Wrong

Early boot is a bad place to assume:

- randomness is plentiful
- libraries never block on entropy
- any transaction ID generation is harmless

In practice, hidden entropy dependencies can cause weird stalls or timing-dependent failures.

### 9.2 What We Added

Relevant files:

- [`internal/entropy/entropy.go`](internal/entropy/entropy.go)
- [`internal/kmod/kmod.go`](internal/kmod/kmod.go)
- [`cmd/mkinitramfs/main.go`](cmd/mkinitramfs/main.go)

We added:

- QEMU `virtio-rng`
- guest-side visibility into entropy state
- module loading support where needed

### 9.3 Why This Mattered Beyond Correctness

It improved confidence.

Once we knew the guest had a real entropy source, debugging other early-boot behavior became much easier because we could stop suspecting weak randomness in every weird edge case.

### 9.4 The Important Project Lesson

If your tiny guest does anything cryptographic, session-oriented, provider-facing, or protocol-heavy, entropy is not optional plumbing. It is core infrastructure.

---

## 10. SSH and the Terminal App

The SSH layer is where the project stopped being just a systems demo and started feeling like a product.

### 10.1 Why Wish

Wish fit the project well because it provides:

- an SSH server in Go
- terminal app integration
- no OpenSSH userland dependency

Relevant files:

- [`internal/sshapp/server.go`](internal/sshapp/server.go)
- [`internal/sshbbs/middleware.go`](internal/sshbbs/middleware.go)

### 10.2 What the SSH Server Does

At a high level:

```text
SSH connection
  -> Wish server accepts session
  -> middleware creates Bubble Tea app session
  -> user lands in BBS
  -> BBS can switch to JS REPL or AI chat
```

### 10.3 The Role of Persistence Here

The SSH server made persistence feel urgent because unstable host keys are immediately annoying:

- every reboot looks like a new host
- clients warn every time
- trust never stabilizes

That is why persistent storage became a priority after SSH existed.

### 10.4 What Still Feels Incomplete

The current service is real, but not yet hardened:

- public-key auth is not the main story yet
- richer user/account management is not there
- it still feels like an appliance app, not a multi-user server

That is okay for now, but it matters for future planning.

---

## 11. The BBS

The BBS is the center of gravity of the interactive system.

Relevant files:

- [`internal/bbsapp/model.go`](internal/bbsapp/model.go)
- [`internal/bbsstore/store.go`](internal/bbsstore/store.go)
- [`cmd/bbs/main.go`](cmd/bbs/main.go)

### 11.1 Why the BBS Was a Good Direction

The BBS gave the project:

- a real terminal UX target
- a reason to care about persistence
- a reason to care about host/guest shared state
- a reason to add the JS REPL
- a natural place to embed AI chat

It turned a systems exercise into something that users can actually poke at.

### 11.2 BBS Modes

The model has four main modes:

- browse
- compose
- JS REPL
- AI chat

This is important because the top-level terminal UX is one application with sub-surfaces, not a pile of separate binaries.

### 11.3 Host-Native BBS

One of the best design decisions in the repo was making the BBS runnable on the host directly:

- [`cmd/bbs/main.go`](cmd/bbs/main.go)

That decision paid off repeatedly because it gave us:

- a faster dev loop
- easier UI debugging
- a shared-state proof that the architecture was not guest-locked

### 11.4 Data Model Simplicity

The BBS store itself is intentionally simpler than the rest of the runtime.

That was the right move. The repo already has enough systems complexity. The BBS persistence layer did not need to be fancy.

---

## 12. The JavaScript REPL

The JS REPL is where the repo’s systems work meets developer ergonomics.

Relevant file:

- [`internal/jsrepl/surface.go`](internal/jsrepl/surface.go)

### 12.1 First Version

The first version was intentionally stripped down:

- evaluate JavaScript
- expose `bbs.*`
- no AST-aware completion
- no real help surfaces

That matched the constraints at the time because the guest runtime was still aggressively CGO-free.

### 12.2 Current Version

The current version uses:

- `go-go-goja`
- the upstream Bobatea adapter
- tree-sitter-backed completion and contextual help

The REPL now supports:

- runtime evaluation
- BBS-specific globals
- completion popup
- help bar
- help drawer
- command palette

### 12.3 Current Key Bindings

The important user-facing bindings in the current guest are:

- `Tab` = trigger completion
- `Enter` = accept selected completion or submit input
- `Alt+H` = open contextual docs drawer
- `Ctrl+P` = command palette
- `Ctrl+T` = toggle focus between input and timeline

We explicitly set those in the REPL config after seeing stale or older guests behave ambiguously around `Tab`.

### 12.4 What We Verified

We verified the new REPL behavior:

- in the host-native BBS
- in a fresh tmux-backed QEMU guest
- over an actual SSH connection into that guest

This was an important lesson in itself: REPL behavior was one of the easiest places to confuse "source state" with "deployed guest state".

### 12.5 What It Still Does Not Do

The current REPL is much better than the original one, but it is not a full terminal editor. The main missing thing is:

- full inline syntax coloring in the input widget itself

It does have good contextual assistance, which is the more important part for now.

---

## 13. The AI Chat Surface

Relevant files:

- [`internal/aichat/surface.go`](internal/aichat/surface.go)
- [`internal/aichat/debug.go`](internal/aichat/debug.go)
- [`internal/aichat/persistence.go`](internal/aichat/persistence.go)

### 13.1 What the Chat Is

The AI chat is an embedded terminal chat surface living inside the BBS runtime.

It is not a separate daemon. It is an app surface inside the same guest environment.

### 13.2 How It Gets Context

The chat currently knows about BBS posts through a seed turn built from recent messages.

That means:

- it sees board state as prompt context
- it does not perform live retrieval on every turn

This is good enough for the current product shape, but it is important to know the difference.

### 13.3 Why Debug Endpoints Became Necessary

The chat stack had multiple failure sources:

- missing config file
- missing profiles file
- missing API key resolution
- broken TLS trust
- HTTP failures
- UI-level failure propagation bugs

Without explicit debug endpoints, those failures blurred together.

The debug endpoints made the system much more understandable because they let us ask:

- what config did the guest actually see?
- what profile did it resolve?
- can the guest perform HTTPS successfully?

### 13.4 The Most Important Chat Lesson

When something "hangs" in an AI client, it is often not one problem. It can be:

- config path bug
- trust root bug
- networking bug
- log noise masking the UI
- a backend finishing without surfacing an explicit error

This repo hit more than one of those.

---

## 14. SQLite, Logging, and the CGO Pivot

This was the biggest design tradeoff in the repository.

### 14.1 The Original Desire

Keep the guest runtime:

- tiny
- static
- pure Go

That worked for a while.

### 14.2 The Pressure

As soon as we wanted richer upstream reuse for:

- timelines
- turns
- logs
- tree-sitter-backed REPL support

the cost of staying purist started to rise.

### 14.3 The Decision

We decided to reuse upstream CGO-backed SQLite stores and support the richer runtime they required.

That added:

- a dynamic loader
- libc and shared library staging
- more initramfs complexity

But it reduced:

- local duplication
- divergence from upstream models
- feature reimplementation pressure

### 14.4 This Was a Real Tradeoff

Good:

- faster feature integration
- upstream-compatible persistence
- richer system behavior

Bad:

- less conceptual purity
- more packaging complexity
- more ways a boot can fail before `/init` executes

### 14.5 Why I Still Think It Was Reasonable

Because the repo’s actual goal stopped being "smallest possible binary" and became "small coherent appliance with useful interactive features."

Once that changed, the CGO decision became defensible.

---

## 15. File-by-File System Tour

This section is intentionally repetitive. If you are trying to orient yourself in the repo, repetition helps.

### 15.1 Build and Packaging

- [`Makefile`](Makefile)
  - top-level orchestration
  - builds the guest runtime
  - builds the initramfs
  - creates storage images
  - stages shared config
  - launches QEMU
- [`cmd/mkinitramfs/main.go`](cmd/mkinitramfs/main.go)
  - builds the archive
  - stages files, modules, and runtime libs
- [`internal/initramfs/writer.go`](internal/initramfs/writer.go)
  - low-level `newc` archive writing
- [`scripts/collect-elf-runtime.sh`](scripts/collect-elf-runtime.sh)
  - extracts ELF runtime dependencies for the CGO guest

### 15.2 Guest Runtime

- [`cmd/init/main.go`](cmd/init/main.go)
  - orchestration entry point
- [`internal/boot/boot.go`](internal/boot/boot.go)
  - early mount behavior and system prep
- [`internal/kmod/kmod.go`](internal/kmod/kmod.go)
  - module load helpers
- [`internal/storage/storage.go`](internal/storage/storage.go)
  - guest-local ext4 mount
- [`internal/sharedstate/sharedstate.go`](internal/sharedstate/sharedstate.go)
  - host-shared `9p` mount
- [`internal/networking/network.go`](internal/networking/network.go)
  - DHCP, routes, DNS
- [`internal/entropy/entropy.go`](internal/entropy/entropy.go)
  - entropy observation and reporting

### 15.3 HTTP and Debug Surfaces

- [`internal/webui/site.go`](internal/webui/site.go)
  - serves the embedded site
  - exposes guest status and debug APIs

### 15.4 SSH and Terminal UX

- [`internal/sshapp/server.go`](internal/sshapp/server.go)
  - SSH server config and host-key handling
- [`internal/sshbbs/middleware.go`](internal/sshbbs/middleware.go)
  - bridges Wish sessions into the Bubble Tea BBS
- [`internal/bbsapp/model.go`](internal/bbsapp/model.go)
  - top-level terminal UI state machine

### 15.5 REPL and Chat

- [`internal/jsrepl/surface.go`](internal/jsrepl/surface.go)
  - JS REPL integration
- [`internal/aichat/surface.go`](internal/aichat/surface.go)
  - AI chat integration
- [`internal/aichat/debug.go`](internal/aichat/debug.go)
  - AI runtime debugging
- [`internal/zlog/config.go`](internal/zlog/config.go)
  - logging defaults and noise control

### 15.6 Persistence

- [`internal/bbsstore/store.go`](internal/bbsstore/store.go)
  - BBS message store
- [`internal/aichat/persistence.go`](internal/aichat/persistence.go)
  - timelines and turns
- [`internal/logstore/store.go`](internal/logstore/store.go)
  - app log persistence
- [`cmd/importqemulogs/main.go`](cmd/importqemulogs/main.go)
  - host QEMU log import

---

## 16. Ticket Timeline and Recommended Historical Reading

If you want to reconstruct how the repo evolved, read these in order.

### QEMU-GO-INIT-001

The initial appliance:

- [`ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/index.md`](ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/index.md)
- [`ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/reference/01-diary.md`](ttmp/2026/03/08/QEMU-GO-INIT-001--single-binary-go-init-that-serves-a-webpage-in-qemu/reference/01-diary.md)

### QEMU-GO-INIT-002

Userspace DHCP and packet-capture debugging:

- [`ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/index.md`](ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/index.md)
- [`ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/reference/01-diary.md`](ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/reference/01-diary.md)
- [`ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/playbook/01-dhcp-packet-capture-and-inspection-playbook.md`](ttmp/2026/03/08/QEMU-GO-INIT-002--userspace-dhcp-in-the-go-init-binary-for-qemu-guest-networking/playbook/01-dhcp-packet-capture-and-inspection-playbook.md)

### QEMU-GO-INIT-003

Entropy support and the first full-system writeup:

- [`ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/index.md`](ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/index.md)
- [`ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/reference/01-diary.md`](ttmp/2026/03/08/QEMU-GO-INIT-003--add-explicit-early-boot-entropy-support-and-diagnostics-for-the-qemu-go-init-runtime/reference/01-diary.md)

### QEMU-GO-INIT-004 and 005

SSH service and then persistence:

- [`ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/index.md`](ttmp/2026/03/08/QEMU-GO-INIT-004--research-and-prototype-a-wish-based-self-hosted-ssh-app-for-the-go-init-runtime/index.md)
- [`ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/index.md`](ttmp/2026/03/08/QEMU-GO-INIT-005--add-persistent-guest-storage-for-host-keys-and-app-state-in-qemu-go-init/index.md)

### QEMU-GO-INIT-006 and 007

Shared-state BBS and JS REPL:

- [`ttmp/2026/03/08/QEMU-GO-INIT-006--design-a-persistent-sqlite-backed-bubble-tea-bbs-for-host-and-ssh-use/index.md`](ttmp/2026/03/08/QEMU-GO-INIT-006--design-a-persistent-sqlite-backed-bubble-tea-bbs-for-host-and-ssh-use/index.md)
- [`ttmp/2026/03/08/QEMU-GO-INIT-007--add-a-javascript-repl-to-the-bubble-tea-bbs-using-go-go-goja-and-bobatea/index.md`](ttmp/2026/03/08/QEMU-GO-INIT-007--add-a-javascript-repl-to-the-bubble-tea-bbs-using-go-go-goja-and-bobatea/index.md)

### QEMU-GO-INIT-008 and 009

AI chat debugging and the CGO persistence pivot:

- [`ttmp/2026/03/09/QEMU-GO-INIT-008--debug-pinocchio-chat-connectivity-and-expose-profile-registry-inspection-endpoints/index.md`](ttmp/2026/03/09/QEMU-GO-INIT-008--debug-pinocchio-chat-connectivity-and-expose-profile-registry-inspection-endpoints/index.md)
- [`ttmp/2026/03/09/QEMU-GO-INIT-009--migrate-guest-chat-persistence-and-logging-to-upstream-cgo-backed-sqlite-stores/index.md`](ttmp/2026/03/09/QEMU-GO-INIT-009--migrate-guest-chat-persistence-and-logging-to-upstream-cgo-backed-sqlite-stores/index.md)

If you only read three detailed historical docs, read:

1. the DHCP playbook
2. the entropy/full-system docs
3. the CGO persistence migration diary

Those three together explain most of the repo’s personality.

---

## 17. What Went Well

This section is intentionally blunt.

### 17.1 What Was Strong Technically

- The repo kept a clear through-line from the first ticket to the current system.
- Build logic stayed concentrated in [`Makefile`](Makefile), which made operational behavior legible.
- Networking became explicit instead of magical.
- Entropy became explicit instead of assumed.
- Storage responsibilities were split cleanly between guest-local persistence and host/guest shared state.
- Wish was a good fit for the SSH surface.
- The BBS was the right product container for adding REPL and chat.
- Reusing upstream persistence avoided a lot of local reinvention.
- Adding debug endpoints dramatically improved our ability to reason about the live guest.

### 17.2 What Was Strong Process-Wise

- Detailed ticket docs were maintained throughout the work.
- Diaries and playbooks captured real debugging context, not just final conclusions.
- We repeatedly validated both host-native and guest-SSH paths instead of trusting one path alone.
- We caught the difference between source state and deployed guest state more than once before it caused worse confusion.

---

## 18. What Went Badly

This is the uncomfortable but useful part.

### 18.1 Repeated Wrong Assumptions

We repeatedly assumed something from a richer normal Linux environment would "just exist" in the tiny guest. Examples:

- kernel `ip=dhcp` support
- early-boot entropy behavior
- CA root availability
- obvious UI failure propagation
- the currently running VM matching the latest source

All of those assumptions turned out to be unsafe.

### 18.2 The Repo Crossed a Complexity Threshold

There was a point where the project stopped being a cute single-binary demo and became a real miniature system with:

- boot logic
- storage logic
- app logic
- interactive terminal logic
- external provider logic
- persistence logic

That complexity is still manageable, but it is real. Pretending otherwise would be a mistake.

### 18.3 Some Feedback Loops Were Still Too Slow

The host-native BBS helped a lot, but anything requiring:

- rebuilding the initramfs
- restarting QEMU
- reconnecting via SSH

still creates friction.

That friction is partly why stale guest state confused us more than once.

### 18.4 CGO Was the Right Choice and Also a Loss

It unlocked the richer storage/runtime path we wanted, but it also ended the cleanest version of the original idea.

That is not failure. It is a trade.

Still, it is worth naming clearly: the project is no longer "just a static Go binary pretending to be an OS."

---

## 19. Biggest Technical Lessons

### 19.1 A Tiny Guest Needs Radical Explicitness

When there is no distro userland to save you, you must explicitly account for:

- boot order
- filesystem availability
- module availability
- entropy
- certificates
- networking
- persistent storage
- shared storage
- dynamic loaders and shared libraries

This repo gradually learned that the hard way.

### 19.2 Debug Surfaces Are Worth Their Weight in Gold

The biggest improvements in debugging quality came not from clever code, but from adding observability:

- packet capture
- runtime endpoints
- structured logs
- explicit state reporting

That pattern should continue.

### 19.3 Host vs Guest Is the Permanent Mental Split

If you get confused in this repository, the first question should be:

> am I talking about the host or the guest?

The second question should be:

> am I looking at source/build state or a currently running VM?

Those two questions solve a surprising amount of confusion.

### 19.4 Reuse Changes Architecture

Using upstream code is not just a dependency decision. It changes:

- build assumptions
- packaging assumptions
- operational assumptions
- failure modes

That happened very clearly with both SQLite reuse and tree-sitter-backed REPL support.

---

## 20. Recommendations for a New Intern

Here is the practical advice I would give an intern before they touch anything.

### 20.1 Understand These Files First

Read these in order:

1. [`README.md`](README.md)
2. [`Makefile`](Makefile)
3. [`cmd/init/main.go`](cmd/init/main.go)
4. [`internal/networking/network.go`](internal/networking/network.go)
5. [`internal/storage/storage.go`](internal/storage/storage.go)
6. [`internal/sharedstate/sharedstate.go`](internal/sharedstate/sharedstate.go)
7. [`internal/bbsapp/model.go`](internal/bbsapp/model.go)
8. [`internal/jsrepl/surface.go`](internal/jsrepl/surface.go)
9. [`internal/aichat/surface.go`](internal/aichat/surface.go)

### 20.2 Use This Validation Habit

When you change something:

1. run targeted tests
2. rebuild `build/init`
3. rebuild `build/initramfs.cpio.gz`
4. start a fresh QEMU instance on fresh ports
5. verify both web and SSH paths
6. if relevant, verify the host-native BBS path too

### 20.3 Keep These Failure Categories in Mind

Any bug you hit is probably one of these:

- build issue
- packaging issue
- boot-order issue
- mount/storage issue
- host/guest path mismatch
- networking issue
- TLS/certs issue
- stale VM issue
- UI routing issue

If you classify the bug correctly early, the repo is much easier to work in.

---

## 21. Future Work That Actually Makes Sense

There is a lot that could be done. Not all of it is equally valuable.

### Highest-Value Next Steps

- public-key SSH auth as the normal path
- better command palette commands in the REPL/BBS
- explicit tool exposure inside the AI chat
- clearer schema/version handling for persisted SQLite stores
- stronger smoke coverage for the guest chat path

### Medium-Value Next Steps

- richer inline JS editing behavior
- more polished host-vs-guest state diagnostics
- better automated restart/test helpers for fresh QEMU sessions

### Lower-Value Next Steps

- chasing ideological purity around static-only binaries

That goal stopped being the most important one once the interactive product surfaces became real.

---

## 22. Final Postmortem

Here is the shortest honest postmortem.

We set out to build a single-binary Go init process that could boot under QEMU and serve a webpage. We succeeded, but the interesting part is what happened next.

The project kept finding the boundaries of the original simplification:

- networking was not simple
- entropy was not simple
- persistence was not simple
- trust roots were not simple
- SSH was not simple
- chat was not simple
- source-to-running-VM deployment was not simple

Every time we hit one of those boundaries, we had two options:

1. pretend the problem was unusual and patch around it
2. accept that the repo needed to grow into a more honest tiny system

The best decisions in the project came from choosing option 2.

That is why the repo is good now.

It is not good because it stayed perfectly pure.
It is good because it stayed coherent while becoming more realistic.

The final system is not the smallest version of the original idea.
It is the most useful version of the original idea that we managed to build.

That is a better outcome.

---

## 23. Appendix: Practical Commands

### Build and Test

```bash
make test
make build
make initramfs KERNEL_IMAGE=qemu-vmlinuz
```

### Run Guest

```bash
make run KERNEL_IMAGE=qemu-vmlinuz QEMU_HOST_PORT=18086 QEMU_SSH_HOST_PORT=10028
```

### Run Host-Native BBS

```bash
go run ./cmd/bbs -state-root build/shared-state/bbs
```

### SSH Into Guest

```bash
ssh -tt \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o PreferredAuthentications=none \
  -o PubkeyAuthentication=no \
  -o PasswordAuthentication=no \
  -p 10028 \
  127.0.0.1
```

### Useful REPL Keys

- `x` from the BBS enters the JS REPL
- `Tab` triggers completion
- `Alt+H` opens docs
- `Ctrl+P` opens the palette
- `Ctrl+T` toggles focus
- `Ctrl+B` returns from sub-surfaces to the BBS

### Useful Questions to Ask When Something Is Wrong

- Did I rebuild the initramfs?
- Did I restart QEMU?
- Am I on the right forwarded ports?
- Is this host state or guest state?
- Is this shared-state storage or guest-local storage?
- Is this a transport failure, a trust failure, or a UI failure?

If you ask those questions first, you will debug this repository much faster.
