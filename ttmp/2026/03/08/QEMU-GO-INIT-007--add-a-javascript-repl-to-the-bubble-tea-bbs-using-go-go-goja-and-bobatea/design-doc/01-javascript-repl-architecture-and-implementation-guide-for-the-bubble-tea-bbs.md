---
Title: JavaScript REPL architecture and implementation guide for the Bubble Tea BBS
Ticket: QEMU-GO-INIT-007
Status: active
Topics:
    - go
    - qemu
    - linux
    - initramfs
    - ssh
    - tui
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T19:32:06.689408719-04:00
WhatFor: "Describe how to embed a Bobatea/go-go-goja JavaScript REPL mode into the existing Bubble Tea BBS for both host-native and Wish-over-SSH execution."
WhenToUse: "Read before changing the BBS architecture, REPL lifecycle, Bubble Tea program ownership, or the local dependency strategy for go-go-goja and bobatea."
---

# JavaScript REPL architecture and implementation guide for the Bubble Tea BBS

## Executive Summary

The existing system already has three important pieces:

- a SQLite-backed message board (`internal/bbsstore`)
- a Bubble Tea application (`internal/bbsapp`)
- a Wish-over-SSH wrapper (`internal/sshbbs`, `internal/sshapp`)

This ticket adds a fourth piece: an embedded JavaScript REPL mode backed by `go-go-goja` for evaluation and `bobatea` for the transcript-oriented REPL UI. The BBS remains the outer shell. The REPL is not a separate binary and not a subprocess. It becomes another application mode, similar to the existing browse and compose modes.

The critical architectural constraint is that Bobatea’s REPL model depends on an event bus and a UI forwarder that must be wired to the actual `*tea.Program`. That requirement affects both:

- the host-native `cmd/bbs` launch path
- the Wish SSH path, which currently uses `wishbubbletea.Middleware(func(sess) (tea.Model, []tea.ProgramOption))`

Because the Wish default middleware hides the `*tea.Program`, the SSH path must move to `wishbubbletea.MiddlewareWithProgramHandler(...)`.

## Problem Statement

The current BBS is useful as a small shared-state message board, but it has no built-in programmable surface. The new requirement is to make the BBS itself a richer operator console:

- SSH into the guest and immediately get a BBS that can also evaluate JavaScript.
- Run the same BBS binary on the host and use the same REPL mode there.
- Keep the shared-state persistence model intact.
- Avoid introducing a traditional userland shell or external tools inside the guest.

There are also two design questions hidden inside this:

1. How do we integrate a Bobatea REPL into an existing Bubble Tea app without splitting the TUI in half?
2. Do Charm libraries need an external terminfo or termios database to render properly?

Answer to the second question:

- Bubble Tea, Lip Gloss, Wish, and Bobatea are self-contained Go libraries for the logic we need.
- They use terminal capabilities exposed by the SSH/client environment and the underlying terminal streams.
- We do not need to ship a separate terminfo database inside the initramfs for basic rendering in this project.
- Rendering quality still depends on the client terminal, but there is no missing “database package” prerequisite comparable to a ncurses deployment.

## Proposed Solution

### High-level shape

Embed a new REPL subsystem into the BBS application:

```text
Host terminal or SSH PTY
        |
        v
  Bubble Tea Program
        |
        v
  BBS App Model
   |    |     |
   |    |     +-- Compose mode
   |    +-------- Browse mode
   +------------- JavaScript REPL mode
                    |
                    v
             Bobatea REPL Model
                    |
          +---------+---------+
          |                   |
          v                   v
   go-go-goja evaluator   Bobatea event bus
                              |
                              v
                    timeline UI forwarder -> tea.Program.Send(...)
```

### Ownership boundaries

The BBS app remains responsible for:

- global mode switching
- top-level header/footer/status rendering
- message browsing and composing
- deciding when the REPL is visible

The REPL subsystem is responsible for:

- creating the JavaScript evaluator
- creating the Bobatea in-memory event bus
- registering `repl.RegisterReplToTimelineTransformer`
- registering `timeline.RegisterUIForwarder` once a concrete `*tea.Program` exists
- closing evaluator resources when the app shuts down

### Suggested package split

- `internal/jsrepl`
  - owns Bobatea/go-go-goja integration
  - exports a small wrapper that the BBS model can embed
- `internal/bbsapp`
  - adds a new mode and hands window/input events to the REPL subsystem
- `cmd/bbs`
  - constructs the top-level BBS model as a pointer and attaches the `*tea.Program`
- `internal/sshbbs`
  - switches to a program-handler-based Wish setup

### Pseudocode

```go
type REPLSurface struct {
    bus       *eventbus.Bus
    evaluator *jsadapter.JavaScriptEvaluator
    model     *repl.Model
    cancel    context.CancelFunc
}

func NewREPLSurface() (*REPLSurface, error) {
    evaluator := jsadapter.NewJavaScriptEvaluatorWithDefaults()
    bus := eventbus.NewInMemoryBus()
    repl.RegisterReplToTimelineTransformer(bus)
    model := repl.NewModel(evaluator, replConfig(), bus.Publisher)
    return &REPLSurface{bus: bus, evaluator: evaluator, model: model}, nil
}

func (r *REPLSurface) AttachProgram(ctx context.Context, p *tea.Program) {
    timeline.RegisterUIForwarder(r.bus, p)
    go r.bus.Run(ctx)
}

type BBSModel struct {
    mode mode
    repl *jsrepl.Surface
}

func (m *BBSModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if m.mode == modeREPL {
        if key == "ctrl+b" {
            m.mode = modeBrowse
            return m, nil
        }
        _, cmd := m.repl.Model().Update(msg)
        return m, cmd
    }
    if key == "x" {
        m.mode = modeREPL
        return m, nil
    }
    ...
}
```

### UI behavior

Recommended initial key map:

- `x`: enter REPL mode from browse mode
- `ctrl+b`: leave REPL mode and return to browse mode
- keep Bobatea’s own keys intact inside REPL mode

That preserves one important rule: the embedded REPL should behave like Bobatea expects once it has focus, instead of the outer BBS hijacking normal keys such as `tab`, `ctrl+h`, or `esc`.

## Design Decisions

### Decision: embed the REPL as a BBS mode

Reasoning:

- avoids process management complexity
- keeps a single SSH session and one Bubble Tea program
- lets the BBS and REPL share layout context and persistent state root

### Decision: keep the BBS as the top-level model

Reasoning:

- the BBS already owns the project-specific identity and navigation
- the REPL is an advanced feature, not the whole product
- easier to preserve host and SSH parity

### Decision: create a wrapper package instead of importing Bobatea/go-go-goja directly in `internal/bbsapp`

Reasoning:

- isolates third-party-ish local integrations
- makes the BBS model easier to read
- gives one place to manage evaluator lifecycle and bus wiring

### Decision: use local `replace` directives

Reasoning:

- the user explicitly wants the local corporate-headquarters packages
- it keeps experiments and integration faithful to the requested dependencies
- the resulting coupling should be documented clearly in the ticket docs

### Decision: switch Wish to `MiddlewareWithProgramHandler`

Reasoning:

- `timeline.RegisterUIForwarder` needs the real `*tea.Program`
- the default Wish handler API only returns `(tea.Model, []tea.ProgramOption)`
- the program handler path cleanly exposes program construction

## Alternatives Considered

### Launch a separate JS REPL binary over SSH

Rejected because:

- duplicates app launch logic
- loses the “BBS plus REPL” product shape
- adds more surface to the initramfs and the Wish server

### Replace the BBS entirely with a Bobatea app

Rejected because:

- discards the message board structure we already built
- makes BBS-specific browsing/composition logic secondary
- forces more redesign than the feature needs

### Implement a plain text REPL without Bobatea

Rejected because:

- ignores the user’s explicit package choice
- throws away completion/help/timeline functionality already provided upstream

### Depend on external shell tools or ncurses databases inside the guest

Rejected because:

- the project goal is still “single Go binary plus minimal boot/runtime support”
- the current terminal stack is already sufficient for initial REPL delivery

## Implementation Plan

1. Verify the exact Bobatea/go-go-goja APIs with a ticket-local probe in `scripts/`.
2. Add local `replace` directives and required module dependencies in the repo `go.mod`.
3. Introduce `internal/jsrepl` to wrap evaluator, bus, and UI-forwarder attachment.
4. Refactor `internal/bbsapp`:
   - add `modeREPL`
   - change model ownership as needed so the program can be attached
   - delegate update/view/init paths to the REPL subsystem when active
5. Update `cmd/bbs` to create a pointer model and attach the program before `Run()`.
6. Update `internal/sshbbs` to use `wishbubbletea.MiddlewareWithProgramHandler`.
7. Add tests where practical, then run host and QEMU smoke validation.
8. Finish the docs, diary, changelog, and upload the bundle to reMarkable.

## Open Questions

Open items to verify during implementation:

- whether the repo’s current Bubble Tea/Lip Gloss versions can coexist cleanly with the local Bobatea/go-go-goja dependency graph
- whether the REPL should expose BBS store helpers into JavaScript in this first ticket or stay as a generic JS surface
- whether the SSH path needs explicit renderer sizing tweaks once the embedded REPL is live

## References

- Local Bobatea REPL model: `/home/manuel/code/wesen/corporate-headquarters/bobatea/pkg/repl/model.go`
- Local Bobatea event bus: `/home/manuel/code/wesen/corporate-headquarters/bobatea/pkg/eventbus/eventbus.go`
- Local Bobatea UI forwarder: `/home/manuel/code/wesen/corporate-headquarters/bobatea/pkg/timeline/wm_forwarder.go`
- Local go-go-goja Bobatea adapter: `/home/manuel/code/wesen/corporate-headquarters/go-go-goja/pkg/repl/adapters/bobatea/javascript.go`
- Wish Bubble Tea middleware: `$(go env GOPATH)/pkg/mod/github.com/charmbracelet/wish@v1.4.7/bubbletea/tea.go`
