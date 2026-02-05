# Brainstorm: Slua Si TUI Orchestrator

**Date:** 2026-02-05
**Status:** Ready for planning

---

## What We're Building

A terminal-based dashboard and session orchestrator for managing multiple remote Sprite instances (Fly.io microVMs) running Claude Code. The TUI acts as a **switchboard** — it shows the status of all Sprites, routes notifications when Claude Code needs attention, and makes it fast to jump into any Sprite's console.

**Target environment:** WSL2 (Debian/Ubuntu), standard Linux.

**Core metaphor:** The TUI is a control tower, not a terminal emulator. It delegates interactive work to `sprite console` and focuses on status awareness and navigation.

## Why This Approach

### Architecture: Shell-out Orchestrator

The TUI suspends itself to hand off to `sprite console -s <name>` for interactive sessions, rather than embedding terminal rendering.

**Rationale:**
- `sprite console` already handles terminal emulation, TTY, resize, etc. Rebuilding that in Bubble Tea is unnecessary complexity.
- The TUI's value is **navigation + notifications**, not terminal rendering.
- Complexity budget should go toward state detection and UX polish, not plumbing.
- This mirrors how tools like `lazygit` or `tig` shell out to editors — proven pattern.

**Rejected alternatives:**
- **Embedded terminal mux** — Requires implementing terminal emulation (vt100 parsing, scroll buffers, input routing). Massive complexity for marginal UX gain.
- **Hybrid tmux management** — Adds tmux as a dependency and failure mode. The Sprites CLI already provides session management.

### State Detection: Periodic Polling

Background goroutine polls each Sprite every ~15-30 seconds via `sprite exec -s <name> <status-check>` to detect Claude Code state changes.

**Rationale:**
- Notification latency of 30s is acceptable for the use case — nothing is critically urgent.
- Polling is simpler than maintaining persistent WebSocket connections per Sprite.
- Navigation speed must never be affected by monitoring — polling in a background goroutine achieves this.
- Avoids resource scaling issues with many concurrent WebSocket connections.

### Stack: Go + Bubble Tea

- **Go**: Single binary, excellent concurrency (goroutines for polling), cross-compiles to Linux/WSL2 trivially.
- **Bubble Tea**: Mature TUI framework, Elm architecture matches this use case well.
- **Lip Gloss**: Styling companion to Bubble Tea.
- **sprites-go SDK**: Exists (superfly/sprites-go), provides API client.
- **Cobra**: CLI framework for `slua` command structure.

## Key Decisions

### 1. Navigation Model: Dashboard + Suspend/Resume

- **Main view**: List of all Sprites with status indicators.
- **Enter on a Sprite**: TUI suspends → `sprite console -s <name>` runs full-screen → exit returns to TUI.
- **Search**: Type-to-filter by Sprite name from the dashboard.
- **Close/destroy**: Action from dashboard, with confirmation + optional checkpoint.

### 2. Notification System (Layered)

**MVP:** TUI-only notifications.
- Status indicators change on the dashboard (e.g., a Sprite goes from "working" to "done" or "needs attention").
- Notification bar/line at bottom of dashboard showing recent state changes.
- Terminal bell on state change.

**Post-MVP (configurable):**
- Desktop notifications via `notify-send` (Linux/WSL2).
- Audible alerts for urgent states (permission requests, crashes).
- Per-Sprite notification preferences.

### 3. State Detection Strategy

The poller detects Claude Code state by checking the Sprite:
- **Running/active**: Claude Code process is running, producing output.
- **Completed**: Claude Code process exited (exit code 0), or idle with recent completion output.
- **Needs attention**: Claude Code is waiting for input (permission prompt, question).
- **Errored/crashed**: Non-zero exit, or process not found unexpectedly.

Detection method: `sprite exec -s <name>` to run a lightweight status-check script on the Sprite that inspects process state and recent output.

**Important terminology note:** Avoid using "task" to describe Claude Code's work units, since Claude Code itself uses "task" in its own interface. Use "job", "session", or "work" instead.

### 4. Auto-Checkpointing: On Claude Code Completion

When the poller detects Claude Code has completed a job (process exited cleanly), auto-create a checkpoint via `sprite checkpoint create -s <name>`.

**Rationale:**
- Covers the main risk: losing work when a Sprite goes idle and eventually gets cleaned up.
- Completion is a natural checkpoint boundary — the Sprite is in a known-good state.
- Avoids noisy time-based checkpoints.
- Git commit detection deferred — adds complexity for marginal benefit when completion-based checkpointing already covers the case.

### 5. WSL2 Compatibility

- Go cross-compiles to Linux natively — no WSL2-specific concerns for the binary.
- `notify-send` works in WSL2 with `wslg` or `wslu` bridge.
- `sprite` CLI works the same in WSL2 as native Linux (it's an HTTP/WebSocket client).
- No X11/GUI dependencies in the TUI itself.

## Feature Scope (MVP)

| Feature | In MVP | Notes |
|---------|--------|-------|
| Dashboard listing all Sprites | Yes | Name, status, uptime |
| Search/filter by name | Yes | Type-to-filter |
| Console into Sprite (shell-out) | Yes | `sprite console -s <name>` |
| Close/destroy Sprite | Yes | With confirmation |
| Background state polling | Yes | 15-30s interval |
| TUI notification bar | Yes | State change alerts |
| Auto-checkpoint on completion | Yes | On Claude Code exit |
| Desktop notifications | No | Post-MVP, configurable |
| Sound alerts | No | Post-MVP, configurable |
| Create new Sprite from TUI | No | Post-MVP |
| Duplicate Sprite | No | Future |
| Setup wizard / templates | No | Future |
| Custom polling commands | No | Future |

## Open Questions

1. **Status check command**: What's the most reliable way to detect Claude Code state from `sprite exec`? Likely `pgrep -a claude` + checking recent stdout from the session. Needs prototyping.
2. **sprites-go SDK maturity**: Need to verify it covers exec, checkpoint, and list operations. May need to fall back to shelling out to `sprite` CLI if the SDK has gaps.
3. **Multiple orgs**: Should the dashboard group Sprites by organization? The `sprite` CLI supports `-o <org>`.
4. **Session persistence**: When the TUI is closed and reopened, should it remember which Sprites it was monitoring? Likely yes — a simple config/state file.
5. **Concurrent console sessions**: Can the user have multiple `sprite console` sessions open? Not from a single TUI, but if they run multiple TUI instances or mix TUI + manual CLI.

---

*Brainstorm captured from collaborative session, 2026-02-05.*
