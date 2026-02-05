---
title: "feat: Build Slua Sí TUI Orchestrator for Fly.io Sprite Sessions"
type: feat
date: 2026-02-05
supersedes: docs/plans/2026-02-05-feat-slua-si-sprite-session-orchestrator-plan.md
---

# Slua Sí — TUI Orchestrator for Fly.io Sprite Sessions

> Slua Sí (Irish: "the fairy host") — a host that gathers, moves, and endures.

**Repo:** https://github.com/JPM1118/slua
**Binary:** `slua`
**Target:** WSL2 (Debian/Ubuntu), Linux

This plan supersedes the prior plan (`2026-02-05-feat-slua-si-sprite-session-orchestrator-plan.md`) by incorporating architectural decisions from the brainstorm session. Key changes: polling replaces persistent WebSocket connections, shell-out pattern is the primary interaction model, and the state machine is expanded.

---

## Overview

When running 10+ Fly.io Sprite instances each with Claude Code, there is no way to see all sessions at a glance, get notified when a Sprite needs attention, or safely manage lifecycles.

Existing tools (Claude Squad, Agent of Empires, Agent Deck) assume local agent execution with local git worktrees. They cannot manage Sprites, where the git repo, Claude Code context, and all state live on remote Firecracker microVMs.

Slua Sí is a **control tower, not a terminal emulator**. It shows status, routes notifications, and makes it fast to jump into any Sprite's console. Interactive work is delegated to `sprite console`.

---

## Problem Statement

| Pain point | Current workaround |
|---|---|
| No overview of all running Sprites | Run `sprite list` manually |
| No notifications when Claude Code finishes or needs input | Manually check each Sprite |
| No quick switching between Sprites | Type `sprite console -s name` each time |
| No safe close with checkpoint prompt | Remember to checkpoint before destroy |
| No auto-checkpointing on completion | Manually checkpoint at milestones |

---

## Architecture

### Core Pattern: Shell-out Orchestrator

The TUI suspends itself to hand off to `sprite console -s <name>` for interactive sessions, rather than embedding terminal rendering.

**Why:**
- `sprite console` already handles terminal emulation, TTY, resize — rebuilding in Bubble Tea is unnecessary
- The TUI's value is navigation + notifications, not terminal rendering
- Mirrors proven patterns from `lazygit` and `tig` (shell out to editors)

**Rejected alternatives:**
- Embedded terminal mux — requires vt100 parsing, scroll buffers, input routing. Massive complexity.
- Hybrid tmux management — adds tmux as dependency and failure mode.

### State Detection: Periodic Polling

Background goroutine polls each Sprite every **15 seconds** via `sprite exec` to detect Claude Code state.

**Why:**
- 15s notification latency is acceptable — nothing in this workflow is critically urgent
- Simpler than maintaining persistent WebSocket connections per Sprite
- Avoids resource scaling issues with many concurrent connections
- Navigation speed is never affected — polling runs in a background goroutine

**Rejected alternatives:**
- Persistent WebSocket connections per Sprite — resource exhaustion with 20+ Sprites, reconnection complexity, marginal latency benefit
- Hybrid REST+WebSocket — unnecessary complexity for MVP when polling covers the use case

### Communication Model

All Sprite operations go through the `sprite` CLI, not the Go SDK directly.

**Why:**
- Inherits authentication from `sprite login` (no separate `SPRITES_TOKEN` management)
- CLI already handles edge cases (retries, auth refresh, org selection)
- Reduces dependency surface — if the Go SDK has gaps, the CLI covers them
- If the SDK proves more reliable/performant for specific operations, we can migrate per-operation later

```
┌─ Local Machine ────────────────────────────────┐
│                                                 │
│  slua (Go binary)                               │
│  ├── cmd/           Cobra commands              │
│  ├── internal/                                  │
│  │   ├── sprites/   sprite CLI wrapper          │
│  │   ├── tui/       Bubble Tea models           │
│  │   ├── poller/    background state monitor    │
│  │   └── notify/    notification dispatch       │
│  └── main.go                                    │
│                                                 │
│  All Sprite operations via: sprite <command>    │
│  Interactive sessions via: sprite console -s X  │
└────────────────────┬────────────────────────────┘
                     │ sprite CLI (HTTP/WebSocket)
┌─ Fly.io ──────────┴────────────────────────────┐
│  Sprite A (Firecracker microVM) — Claude Code   │
│  Sprite B (Firecracker microVM) — Claude Code   │
│  Sprite C (Firecracker microVM) — Claude Code   │
└─────────────────────────────────────────────────┘
```

---

## State Machine

Each Sprite has one of these states, detected by the poller:

```
                    session active
  ┌──────────┐  ──────────────────>  ┌──────────┐
  │ SLEEPING │                       │ WORKING  │
  │  (dim)   │  <──────────────────  │ (yellow) │
  └──────────┘    no active session  └──────────┘
       ↑                                │     │
       │         exit code 0            │     │ prompt/question detected
       │    ┌────────────┐              │     │
       └────│  FINISHED  │ <───────────┘     │
            │  (green)   │                   │
            └────────────┘             ┌──────────┐
                                       │ WAITING  │
                                       │  (red)   │
                                       └──────────┘

  Transient states (shown with spinner):
  ┌───────────────┐  ┌────────────┐  ┌──────────────┐
  │ CHECKPOINTING │  │ DESTROYING │  │   CREATING   │
  └───────────────┘  └────────────┘  └──────────────┘

  Error states:
  ┌─────────────┐  ┌─────────┐
  │ UNREACHABLE │  │  ERROR  │
  │   (dim ?)   │  │  (red !)│
  └─────────────┘  └─────────┘
```

### State Definitions

| State | Detection Method | Visual |
|---|---|---|
| SLEEPING | No active exec session, no recent activity | Dim gray text |
| WORKING | Active exec session, Claude Code process running (`pgrep -a claude`) | Yellow text |
| FINISHED | Claude Code process not found, last exit code 0 | Green text |
| WAITING | Claude Code process running, recent stdout contains prompt pattern | Red bold text |
| ERROR | Non-zero exit code, or process disappeared unexpectedly | Red `!` indicator |
| UNREACHABLE | `sprite exec` timed out or returned connection error | Dim `?` indicator |
| CHECKPOINTING | Checkpoint operation in flight (local state, not polled) | Spinner |
| DESTROYING | Destroy operation in flight (local state, not polled) | Spinner |
| CREATING | Create operation in flight (local state, not polled) | Spinner |

### Status Detection Command

The poller runs this on each Sprite every 15 seconds:

```bash
sprite exec -s <name> -- sh -c '
  if pgrep -a claude > /dev/null 2>&1; then
    # Claude Code is running — check if waiting for input
    RECENT=$(tmux capture-pane -p -l 5 2>/dev/null || echo "")
    if echo "$RECENT" | grep -qE "(Y/n|y/N|\? |> $|Permission|Allow|Deny)"; then
      echo "WAITING"
    else
      echo "WORKING"
    fi
  else
    # Claude Code is not running — check exit status
    EXIT=$(tmux show-environment CLAUDE_EXIT 2>/dev/null | cut -d= -f2 || echo "")
    if [ "$EXIT" = "0" ] || [ -z "$EXIT" ]; then
      echo "FINISHED"
    else
      echo "ERROR:$EXIT"
    fi
  fi
'
```

**Fallback:** If the exec command times out (>5s) or fails, state is `UNREACHABLE`. If the output does not match any expected pattern, state is `SLEEPING` (conservative default).

**Configuration:** Prompt detection patterns are configurable in `~/.config/slua/config.yml` so users can adjust for Claude Code output changes:

```yaml
detection:
  poll_interval: 15s
  exec_timeout: 5s
  prompt_patterns:
    - "Y/n"
    - "y/N"
    - "\\? "
    - "> $"
    - "Permission"
    - "Allow"
    - "Deny"
```

---

## Keybindings and Input Model

### Modal Input (vim-style)

The dashboard uses **modal input** to avoid conflicts between navigation and search:

**Normal mode** (default):

| Key | Action |
|---|---|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `Enter` | Connect to selected Sprite (shell-out to `sprite console`) |
| `d` | Destroy selected Sprite (opens confirmation dialog) |
| `c` | Checkpoint selected Sprite |
| `/` | Enter search mode |
| `r` | Force refresh (re-poll all Sprites immediately) |
| `?` | Show help overlay |
| `q` | Quit |

**Search mode** (activated by `/`):

| Key | Action |
|---|---|
| Any character | Type into search filter |
| `Backspace` | Delete last character |
| `Enter` | Exit search mode, keep filter active |
| `Esc` | Clear filter and exit search mode |

**Confirmation dialog** (modal, blocks other input):

| Key | Action |
|---|---|
| `1` | Checkpoint then destroy |
| `2` | Destroy without checkpoint |
| `3` | Cancel |
| `Esc` | Cancel |

---

## Dashboard Layout

```
┌─────────────────────────────────────────────────────────────────────┐
│  Slua Sí                                    [2 need attention] ⚡   │
│  Connected · Last poll: 3s ago                                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  NAME                    STATUS       UPTIME      LAST ACTIVITY     │
│  ─────────────────────── ──────────── ─────────── ────────────────  │
│▸ my-web-app              WAITING      2h 15m      prompt: Y/n       │
│  api-refactor            WORKING      1h 42m      writing code      │
│  docs-update             FINISHED ✓   45m         completed         │
│  auth-system             WORKING      3h 01m      running tests     │
│  billing-fix             SLEEPING     5h 22m      idle              │
│  search-perf             ERROR !      12m         exit code 1       │
│  mobile-app              UNREACHABLE  —           connection lost    │
│                                                                     │
│                                                                     │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  ● my-web-app needs attention (2m ago) │ ● docs-update finished ✓   │
├─────────────────────────────────────────────────────────────────────┤
│  j/k:navigate  Enter:connect  d:destroy  c:checkpoint  /:search    │
└─────────────────────────────────────────────────────────────────────┘
```

### Layout Regions

1. **Header** (2 lines): App name, attention badge count, connection status, last poll timestamp
2. **Sprite list** (fills remaining height minus header/footer): Scrollable list with columns
3. **Notification bar** (1-2 lines): Most recent state change events, FIFO, max 2 visible
4. **Status bar** (1 line): Key binding hints

### Column Definitions

| Column | Width | Content |
|---|---|---|
| NAME | 24 chars, left-aligned, truncated with `…` | Sprite name |
| STATUS | 12 chars | State label with color |
| UPTIME | 11 chars | Duration since Sprite creation (e.g., `2h 15m`) |
| LAST ACTIVITY | 16+ chars, fills remaining width | Human-readable description of last detected state |

### Responsive Behavior

- **Minimum:** 80x24 terminal. Below this, show "Terminal too small (need 80x24)" message.
- **Narrow (<100 cols):** Hide LAST ACTIVITY column.
- **Tall (>30 rows):** Show more Sprites without scrolling.
- **Resize:** Handled by Bubble Tea's `WindowSizeMsg`.

---

## Implementation Phases

### Phase 1: Dashboard + Shell-out (MVP)

**Goal:** See all Sprites, navigate, connect. Prove the core loop works.

**Deliverables:**
- Go module initialized (`github.com/JPM1118/slua`)
- Cobra CLI skeleton: `slua` (default → dashboard), `slua dashboard`, `slua status`
- Sprite CLI wrapper: list, get status (via `sprite list`, `sprite status`)
- Bubble Tea dashboard: list view, keyboard navigation, cursor
- Shell-out: `Enter` suspends TUI, runs `sprite console -s <name>`, resumes on exit
- Basic status display (SLEEPING/WORKING based on `sprite list` output — no exec polling yet)

**Files:**

```
go.mod
go.sum
main.go
cmd/
  root.go           — Cobra root command, defaults to dashboard
  dashboard.go      — launches TUI
  status.go         — non-TUI status dump
internal/
  sprites/
    cli.go          — wraps `sprite` CLI commands (list, status, console)
  tui/
    dashboard.go    — Bubble Tea model: list, navigation, shell-out
    styles.go       — Lip Gloss style definitions
.gitignore
```

**Acceptance criteria:**
- [x] `slua` shows all Sprites with name, basic status, uptime
- [x] `j`/`k` navigation works, `Enter` opens `sprite console`, `q` quits
- [x] Shell-out suspends TUI cleanly; returning restores the dashboard
- [x] `slua status` prints a non-TUI status table
- [x] Works on WSL2 and native Linux

**Estimated scope:** ~400-600 lines of Go

---

### Phase 2: State Polling + Notifications

**Goal:** Know when a Sprite needs attention without manually checking.

**Deliverables:**
- Background poller goroutine: runs status detection command via `sprite exec` every 15s
- State machine: SLEEPING → WORKING → FINISHED/WAITING/ERROR/UNREACHABLE
- Dashboard updates: status column reflects polled state with color
- Notification bar: shows recent state transitions (FIFO, max 2 visible)
- Attention badge: `[N need attention]` in header
- Terminal bell: on transition to WAITING or ERROR (debounced: max 1 bell per 30s)
- Bell suppression during shell-out (poller continues, bells queued, notifications shown on resume)
- Configurable poll interval and prompt patterns (`~/.config/slua/config.yml`)

**Files (new/modified):**

```
internal/
  poller/
    poller.go       — background goroutine, manages poll loop per Sprite
    detector.go     — runs status detection command, parses output
    state.go        — state machine types and transitions
  notify/
    terminal.go     — terminal bell with debounce
    bar.go          — notification bar messages (FIFO queue)
  tui/
    dashboard.go    — (modified) add poller integration, notification bar, badge
    styles.go       — (modified) add state-specific colors
config.go           — config file parsing (~/.config/slua/config.yml)
```

**Acceptance criteria:**
- [ ] Dashboard shows live state (WORKING/WAITING/FINISHED/ERROR/UNREACHABLE) with color
- [ ] State updates within 15s of actual change
- [ ] Terminal bell on WAITING/ERROR transitions (debounced)
- [ ] No bells ring while inside `sprite console`
- [ ] Notification bar shows last 2 state changes with timestamps
- [ ] Badge count in header reflects Sprites needing attention
- [ ] Connecting to a WAITING Sprite clears its notification
- [ ] Poll interval configurable via config file

**Estimated scope:** ~600-800 lines of Go

---

### Phase 3: Safe Lifecycle + Auto-Checkpoint

**Goal:** Never lose work. Checkpoint on completion. Confirm before destroy.

**Deliverables:**
- `d` key: opens confirmation dialog (checkpoint+destroy / destroy / cancel)
- `c` key: creates checkpoint immediately, shows spinner during operation
- Auto-checkpoint: when poller detects transition from WORKING → FINISHED, create checkpoint via `sprite checkpoint create -s <name>`
- Checkpoint naming: auto-checkpoints named `auto-YYYYMMDD-HHMMSS`, manual named `manual-YYYYMMDD-HHMMSS` (or user-provided)
- `slua destroy <name>` CLI command with same confirmation flow
- `slua checkpoint <name>` CLI command
- Error handling: if checkpoint fails, show error in notification bar, do NOT proceed with destroy

**Files (new/modified):**

```
cmd/
  destroy.go        — destroy command with interactive prompt
  checkpoint.go     — checkpoint shortcut command
internal/
  sprites/
    cli.go          — (modified) add checkpoint create, destroy operations
  tui/
    confirm.go      — confirmation dialog Bubble Tea model
    dashboard.go    — (modified) add d/c keybindings, spinner states
  poller/
    poller.go       — (modified) trigger auto-checkpoint on WORKING→FINISHED
```

**Concurrency rules:**
- Destroy request while checkpoint in flight → queue destroy after checkpoint completes
- Checkpoint request while another checkpoint in flight → ignore (show "already checkpointing")
- State change notification while confirmation dialog is open → update notification bar, do not dismiss dialog
- Auto-checkpoint fires but Sprite already checkpointed in last 60s → skip (deduplication)

**Acceptance criteria:**
- [ ] `d` opens confirmation dialog, all three options work
- [ ] Auto-checkpoint fires within 15s of Claude Code completion
- [ ] Auto-checkpoint skipped if already checkpointed in last 60s
- [ ] `c` creates checkpoint with spinner, shows success/error
- [ ] Checkpoint failure blocks destroy and shows error
- [ ] `slua destroy` CLI works outside the TUI

**Estimated scope:** ~400-600 lines of Go

---

### Phase 4: Search, Polish, and Configuration

**Goal:** Production-ready MVP with search, error resilience, and session persistence.

**Deliverables:**
- `/` search mode: type-to-filter by Sprite name, `Esc` to clear
- `r` force refresh: immediately re-poll all Sprites
- `?` help overlay: shows all keybindings
- Session persistence: save dashboard state to `~/.config/slua/state.json` (last cursor position, active filter)
- Error resilience: stale data with "connection lost" warning, automatic retry on poll failure
- API health indicator: connection status in header (Connected / Reconnecting / Disconnected)
- Graceful degradation: if `sprite exec` fails for one Sprite, mark it UNREACHABLE, continue polling others
- Sort: Sprites sorted by status priority (WAITING > ERROR > WORKING > FINISHED > SLEEPING > UNREACHABLE), then alphabetically

**Files (new/modified):**

```
internal/
  tui/
    dashboard.go    — (modified) search mode, help overlay, sort, health indicator
    search.go       — search filter logic
    help.go         — help overlay model
  persist/
    state.go        — save/load dashboard state
```

**Acceptance criteria:**
- [ ] `/` activates search, filters list in real-time, `Esc` clears
- [ ] `r` triggers immediate refresh with visual feedback
- [ ] `?` shows help overlay
- [ ] Dashboard state persists across sessions
- [ ] Single Sprite failure does not crash the dashboard
- [ ] Connection status visible in header
- [ ] Sprites sorted by attention priority

**Estimated scope:** ~400-500 lines of Go

---

## Post-MVP Features (Deferred)

These are explicitly out of scope for the MVP but documented for future planning:

| Feature | Notes |
|---|---|
| Desktop notifications (`notify-send`) | Configurable, requires `wslg`/`wslu` detection on WSL2 |
| Sound alerts | Configurable per-state |
| `slua new` setup wizard | Interactive Sprite creation + GitHub repo + environment template |
| `.slua.yml` templates | Project templates for consistent environments |
| GitHub integration | Repo creation, credential setup on Sprites |
| `slua cycle next/prev` | Quick non-TUI switching |
| Multi-org support | Dashboard groups by org, `-o <org>` flag |
| ntfy push notifications | Phone notifications via ntfy.sh |
| Sprite grouping/tagging | Filter by project/tag |
| Cost tracking | Running costs per Sprite |

---

## Technology Stack

| Component | Choice | Rationale |
|---|---|---|
| Language | Go | Single binary, goroutines for polling, cross-compiles to Linux |
| TUI | Bubble Tea (`charmbracelet/bubbletea`) | Elm architecture, mature, handles resize/suspend |
| Styling | Lip Gloss (`charmbracelet/lipgloss`) | Companion to Bubble Tea |
| CLI | Cobra (`spf13/cobra`) | Standard Go CLI framework |
| Sprites | `sprite` CLI (shell-out) | Inherits auth, handles edge cases; migrate to SDK per-operation if needed |
| Config | YAML (`~/.config/slua/config.yml`) | Poll interval, prompt patterns, notification preferences |

---

## Configuration

### `~/.config/slua/config.yml`

```yaml
# Polling
detection:
  poll_interval: 15s
  exec_timeout: 5s
  prompt_patterns:
    - "Y/n"
    - "y/N"
    - "\\? "
    - "> $"
    - "Permission"
    - "Allow"
    - "Deny"

# Auto-checkpoint
checkpoints:
  auto_on_completion: true
  dedup_window: 60s          # skip if checkpointed within this window
  name_prefix: "auto"

# Notifications
notifications:
  terminal_bell: true
  bell_debounce: 30s         # max 1 bell per this interval
  bell_on_states:
    - WAITING
    - ERROR

# Display
display:
  sort_by: priority           # priority | name | uptime
  min_terminal_width: 80
  min_terminal_height: 24
```

### `~/.config/slua/state.json` (auto-managed)

```json
{
  "last_cursor": 2,
  "last_filter": "",
  "dismissed_notifications": ["sprite-a:FINISHED:2026-02-05T14:30:00Z"]
}
```

---

## Error Handling Strategy

### Principles

1. **Never crash the dashboard.** All errors are caught and displayed, not propagated as panics.
2. **Show stale data with warning, not blank screens.** If polling fails, keep last known state, show "Last updated: Xm ago" warning.
3. **Isolate failures per Sprite.** One unreachable Sprite does not affect monitoring of others.
4. **Retry automatically, alert on persistent failure.** Transient errors get exponential backoff; 3+ consecutive failures trigger UNREACHABLE state.

### Error Display

- **Transient API errors:** Notification bar message: "Failed to poll sprite-x (retrying)"
- **Auth expired:** Header warning: "Authentication expired — run `sprite login`"
- **All Sprites unreachable:** Header warning: "Cannot reach Sprites API — check network"
- **Individual Sprite errors:** Row-level ERROR or UNREACHABLE state indicator
- **Checkpoint/destroy failure:** Notification bar error message, operation does not proceed

### Timeout Values

| Operation | Timeout | On timeout |
|---|---|---|
| `sprite list` | 10s | Show stale data + warning |
| `sprite exec` (status check) | 5s | Mark Sprite as UNREACHABLE |
| `sprite checkpoint create` | 30s | Show error, retry once |
| `sprite destroy` | 15s | Show error, do not retry |
| `sprite console` (shell-out) | None (user-controlled) | — |

---

## Dependencies and Prerequisites

| Dependency | Required | Notes |
|---|---|---|
| `sprite` CLI | Yes | Must be installed and authenticated (`sprite login`) |
| Go 1.22+ | Build only | Not needed at runtime |
| Bubble Tea | Yes | `github.com/charmbracelet/bubbletea` |
| Lip Gloss | Yes | `github.com/charmbracelet/lipgloss` |
| Cobra | Yes | `github.com/spf13/cobra` |
| YAML parser | Yes | `gopkg.in/yaml.v3` |

---

## Risk Analysis

| Risk | Impact | Likelihood | Mitigation |
|---|---|---|---|
| Claude Code output patterns change | State detection wrong | Medium | Configurable patterns in config.yml, fallback to SLEEPING |
| `sprite exec` unreliable for status checks | Polling fails | Low | Timeout + UNREACHABLE state + retry with backoff |
| `sprite console` hangs on exit | TUI stays suspended | Low | User can Ctrl+C; TUI catches SIGINT to resume |
| Bell noise during shell-out | Disruptive UX | Medium | Suppress bells while TUI is suspended |
| Many Sprites (50+) slow the poll loop | Dashboard lag | Low | Parallelize exec calls with goroutine pool (max 10 concurrent) |
| `sprite` CLI not installed | App cannot function | Low | Check on startup, show clear error message |

---

## Directory Structure (Final)

```
slua/
├── main.go
├── go.mod
├── go.sum
├── .gitignore
├── cmd/
│   ├── root.go              # Cobra root, defaults to dashboard
│   ├── dashboard.go         # TUI entry point
│   ├── status.go            # Non-TUI status dump
│   ├── destroy.go           # Destroy with confirmation (Phase 3)
│   └── checkpoint.go        # Quick checkpoint (Phase 3)
├── internal/
│   ├── sprites/
│   │   └── cli.go           # Wraps sprite CLI commands
│   ├── tui/
│   │   ├── dashboard.go     # Main dashboard model
│   │   ├── styles.go        # Lip Gloss styles
│   │   ├── confirm.go       # Confirmation dialog (Phase 3)
│   │   ├── search.go        # Search filter (Phase 4)
│   │   └── help.go          # Help overlay (Phase 4)
│   ├── poller/
│   │   ├── poller.go        # Background poll loop (Phase 2)
│   │   ├── detector.go      # Status detection logic (Phase 2)
│   │   └── state.go         # State machine types (Phase 2)
│   ├── notify/
│   │   ├── terminal.go      # Terminal bell with debounce (Phase 2)
│   │   └── bar.go           # Notification bar messages (Phase 2)
│   ├── persist/
│   │   └── state.go         # Save/load dashboard state (Phase 4)
│   └── config/
│       └── config.go        # Config file parsing (Phase 2)
└── docs/
    ├── research.md
    ├── plans/
    │   └── (this file)
    └── brainstorms/
        └── 2026-02-05-slua-si-tui-orchestrator-brainstorm.md
```

---

## Open Questions (Resolved)

These questions from the brainstorm and SpecFlow analysis are resolved in this plan:

| Question | Resolution |
|---|---|
| Polling vs WebSocket? | Polling every 15s via `sprite exec`. No persistent WebSocket connections. |
| Auth model? | Shell out to `sprite` CLI, inherits its auth. No separate `SPRITES_TOKEN` management. |
| Status detection command? | `sprite exec` running `pgrep` + `tmux capture-pane` (see State Machine section). |
| Keybinding/search conflict? | Modal input: `/` enters search mode, normal mode has single-char keybindings. |
| Multi-org? | Deferred. Single org (whatever `sprite` CLI defaults to). |
| Poller during shell-out? | Poller continues. Bells suppressed. Notifications queued for display on resume. |
| "Detach only" in destroy? | Removed. Destroy dialog has 3 options: checkpoint+destroy, destroy, cancel. |
| Checkpoint naming? | `auto-YYYYMMDD-HHMMSS` for auto, `manual-YYYYMMDD-HHMMSS` for manual. |
| State indicator visuals? | Color-coded text labels (see State Machine section). |
| API error handling? | Show stale data + warning. Per-Sprite error isolation. See Error Handling section. |

## Open Questions (Remaining)

1. **sprites-go SDK coverage:** Need to verify if the Go SDK covers all operations or if shelling out is required for everything. If the SDK works well, migrating high-frequency operations (list, status) from CLI to SDK would reduce overhead.
2. **tmux session structure on Sprites:** The status detection command assumes Claude Code runs in a tmux session. Need to verify the exact session/pane structure on a real Sprite.
3. **Concurrent console sessions:** Can multiple `sprite console` connections exist for the same Sprite? If so, does the second connection conflict?

---

## Quality Gates

- [ ] `go vet` passes
- [ ] `golangci-lint` passes
- [ ] Unit tests for state machine transitions
- [ ] Unit tests for status detection parser
- [ ] Integration test: list → connect → return flow with a real Sprite
- [ ] No hardcoded credentials
- [ ] Works on WSL2 (Debian) and native Linux
- [ ] Handles `sprite` CLI not being installed with clear error message

---

## References

### Internal
- Research: `docs/research.md`
- Brainstorm: `docs/brainstorms/2026-02-05-slua-si-tui-orchestrator-brainstorm.md`
- Prior plan (superseded): `docs/plans/2026-02-05-feat-slua-si-sprite-session-orchestrator-plan.md`

### External
- Sprites API: https://sprites.dev/api
- sprites-go SDK: https://github.com/superfly/sprites-go
- Bubble Tea: https://github.com/charmbracelet/bubbletea
- Claude Squad (reference): https://github.com/smtg-ai/claude-squad
- Agent Deck (reference): https://github.com/asheshgoplani/agent-deck

---

*Plan created 2026-02-05. Supersedes prior plan from same date.*
