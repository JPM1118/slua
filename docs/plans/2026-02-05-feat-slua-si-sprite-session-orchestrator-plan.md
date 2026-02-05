---
title: "feat: Build Slua Sí — TUI orchestrator for Fly.io Sprite sessions"
type: feat
date: 2026-02-05
---

# Slua Sí — TUI Orchestrator for Fly.io Sprite Sessions

**Slua Sí** (Irish: "the fairy host")
A TUI orchestrator for managing many running Sprites at once: start/stop, status, logs, grouping, and lifecycle control.

> Slua Sí: a host that gathers, moves, and endures.

**Repo:** https://github.com/JPM1118/slua
**Sprite:** `slua`
**CLI command:** `slua`

---

## Overview

When running 10+ Fly.io Sprite instances each with Claude Code, there is no way to see all sessions at a glance, get notified when a Sprite needs attention, cycle between sessions, auto-checkpoint, or spin up new Sprites with consistent environments.

Existing tools (Claude Squad, Agent of Empires, Agent Deck) all assume local agent execution with local git worktrees. They do not work for Sprites, where the git repo, Claude Code context, and all state live on remote Firecracker microVMs.

Slua Sí fills this gap: a TUI that manages remote Sprites via the Sprites API, providing a dashboard, bidirectional notifications, safe lifecycle management, and a setup wizard.

---

## Problem Statement

| Pain point | Current workaround |
|------------|-------------------|
| No overview of all running Sprites | Run `sprite list` manually, scan output |
| No notifications when Claude Code finishes or needs input | Keep checking each Sprite manually |
| No quick switching between Sprites | `sprite console -s name` each time |
| No safe close with checkpoint prompt | Remember to `sprite checkpoint create` before destroy |
| No auto-checkpointing | Manually checkpoint at milestones |
| No consistent new-project setup | Manually run sprite-up, create GitHub repo, configure |

---

## Proposed Solution

A Go CLI + TUI application (`slua`) that wraps the Sprites API to provide:

1. **Dashboard** — Live-updating list of all Sprites with status
2. **Notifications** — Bidirectional: watch Sprite output, fire local alerts, send commands back
3. **Safe Lifecycle** — "Save before close?" checkpoint prompts, auto-checkpointing
4. **Setup Wizard** — Interactive new-project flow with GitHub repo creation and environment consistency

---

## Technical Approach

### Technology Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Language | **Go** | Both Claude Squad and Agent Deck use Go + Bubble Tea. Proven for TUI + WebSocket. Single binary distribution. |
| TUI Framework | **Bubble Tea** (charmbracelet/bubbletea) | Elm Architecture, command-based concurrency, battle-tested for AI agent TUIs |
| Styling | **Lip Gloss** (charmbracelet/lipgloss) | Standard companion to Bubble Tea |
| Sprites API | **REST + WebSocket** via sprites-go SDK or direct HTTP | Go SDK available at superfly/sprites-go |
| Desktop Notifications | **beeep** (gen2brain/beeep) | Cross-platform (Linux D-Bus, macOS native, Windows) |
| GitHub Integration | **go-gh** or shell out to `gh` CLI | Repo creation, PR management |
| Config | **YAML** (.slua.yml) | Project templates and environment setup recipes |

### Architecture

```
┌─ Local Machine ─────────────────────────────────────────┐
│                                                         │
│  slua CLI                                               │
│  ├── cmd/           (Cobra commands)                    │
│  │   ├── dashboard  (TUI main screen)                   │
│  │   ├── new        (setup wizard)                      │
│  │   ├── destroy    (safe lifecycle)                    │
│  │   ├── cycle      (next/prev switching)               │
│  │   └── status     (quick status check)                │
│  │                                                      │
│  ├── internal/                                          │
│  │   ├── sprites/   (API client wrapper)                │
│  │   │   ├── client.go       (REST + WebSocket)         │
│  │   │   ├── monitor.go      (output watcher)           │
│  │   │   └── checkpoint.go   (checkpoint management)    │
│  │   │                                                  │
│  │   ├── tui/       (Bubble Tea models)                 │
│  │   │   ├── dashboard.go    (main overview)            │
│  │   │   ├── detail.go       (single Sprite view)       │
│  │   │   ├── wizard.go       (new project flow)         │
│  │   │   └── confirm.go      (lifecycle prompts)        │
│  │   │                                                  │
│  │   ├── notify/    (notification dispatch)             │
│  │   │   ├── desktop.go      (beeep integration)        │
│  │   │   ├── terminal.go     (bell + TUI badge)         │
│  │   │   └── ntfy.go         (optional push to phone)   │
│  │   │                                                  │
│  │   └── github/    (repo management)                   │
│  │       ├── create.go       (gh repo create wrapper)   │
│  │       └── sync.go         (push/pull status)         │
│  │                                                      │
│  └── config/        (.slua.yml templates)               │
│                                                         │
│  Communication:                                         │
│  ├── WebSocket (persistent) → monitor Sprite output     │
│  └── REST (on-demand) → exec, checkpoint, destroy       │
└─────────────────────────────────────────────────────────┘
          ↕ WebSocket streams          ↕ REST API calls
┌─ Fly.io ────────────────────────────────────────────────┐
│  Sprite A (Firecracker microVM) — Claude Code session   │
│  Sprite B (Firecracker microVM) — Claude Code session   │
│  Sprite C (Firecracker microVM) — Claude Code session   │
│  ...                                                    │
└─────────────────────────────────────────────────────────┘
```

### Sprites API Primitives Used

**Monitoring (Sprite → Local):**
- `WSS /v1/sprites/{name}/exec/{session-id}` — attach to session, stream stdout/stderr via binary multiplexed protocol (0x01=stdout, 0x02=stderr, 0x03=exit)
- `GET /v1/sprites/{name}/exec` — list sessions with metadata: `is_active`, `bytes_per_second`, `last_activity`
- Scrollback buffer on reattach — see output that happened while disconnected

**Commands (Local → Sprite):**
- `POST /v1/sprites/{name}/exec` — fire commands (checkpoint triggers, process management)
- `POST /v1/sprites/{name}/checkpoints` — create checkpoint via REST
- `POST /v1/sprites/{name}/checkpoints/{id}/restore` — restore checkpoint
- `DELETE /v1/sprites/{name}` — destroy Sprite
- `POST /v1/sprites` — create Sprite

**Auth:** Bearer token (`SPRITES_TOKEN` env var) on all requests.

### State Detection Strategy

Since the Sprites API has no webhook/event system for state changes, Slua uses a hybrid approach:

1. **Lightweight polling** — Every 5s, `GET /v1/sprites/{name}/exec` for all tracked Sprites. Check `is_active`, `bytes_per_second`, `last_activity` to classify state.
2. **Content-based detection** — For Sprites marked as "active," maintain a WebSocket connection and watch stdout for patterns:
   - Claude Code waiting for input: prompt patterns (`?`, `Y/n`, `>`)
   - Claude Code finished: exit code received (0x03 frame)
   - Claude Code error: stderr output patterns
3. **State machine per Sprite:**

```
┌──────────┐   session active    ┌──────────┐
│ SLEEPING │ ──────────────────> │ WORKING  │
└──────────┘                     └──────────┘
      ↑                            │      │
      │        exit code           │      │ prompt detected
      │  ┌──────────────┐         │      │
      └──│   FINISHED   │ <──────┘      │
         └──────────────┘               │
                                  ┌──────────┐
                                  │ WAITING  │
                                  │ (input)  │
                                  └──────────┘
```

---

## Implementation Phases

### Phase 1: Foundation (MVP)

**Goal:** Basic dashboard + manual cycling. Prove the API integration works.

**Deliverables:**
- `slua` binary with Cobra CLI skeleton
- `slua dashboard` — Bubble Tea TUI showing all Sprites (name, state, uptime)
- `slua status` — Quick non-TUI status dump
- Sprites API client (REST: list, get, create, destroy; auth via bearer token)
- Keyboard navigation: `j`/`k` to move, `Enter` to connect (launches `sprite console`), `q` to quit
- `slua cycle next` / `slua cycle prev` — non-TUI quick switch

**Files to create:**
- `cmd/root.go` — Cobra root command
- `cmd/dashboard.go` — dashboard command
- `cmd/status.go` — status command
- `cmd/cycle.go` — cycle next/prev command
- `internal/sprites/client.go` — REST API client
- `internal/tui/dashboard.go` — Bubble Tea dashboard model
- `go.mod`, `go.sum`

**Success criteria:**
- [ ] `slua dashboard` shows all active Sprites with live status
- [ ] Can navigate and select a Sprite to connect
- [ ] `slua cycle next` connects to the next Sprite in the list
- [ ] Status correctly reflects Sprite state (running, sleeping)

**Estimated scope:** ~500-800 lines of Go

---

### Phase 2: Notifications (Bidirectional)

**Goal:** Know when a Sprite needs attention without looking at the dashboard.

**Deliverables:**
- WebSocket monitor: persistent connections to active Sprite sessions
- Output parser: detect Claude Code state from stdout patterns
- Desktop notifications via beeep (Linux/macOS/Windows)
- Terminal bell as fallback
- TUI badge: `[3 need attention]` on dashboard header
- Dashboard row highlighting for Sprites needing attention

**Files to create/modify:**
- `internal/sprites/monitor.go` — WebSocket session monitor
- `internal/sprites/parser.go` — stdout pattern matcher for Claude Code states
- `internal/notify/desktop.go` — beeep wrapper
- `internal/notify/terminal.go` — terminal bell
- `internal/tui/dashboard.go` — add notification badges + row highlighting

**Success criteria:**
- [ ] Dashboard shows real-time status (WORKING / WAITING / FINISHED)
- [ ] Desktop notification fires when a Sprite transitions to WAITING or FINISHED
- [ ] Badge count updates in real-time on dashboard header
- [ ] Connecting to a notified Sprite clears the notification

**Estimated scope:** ~600-900 lines of Go

---

### Phase 3: Safe Lifecycle ("Save Before Close?")

**Goal:** Never accidentally lose work when destroying a Sprite.

**Deliverables:**
- `slua destroy <name>` — interactive prompt with options:
  1. Checkpoint, then destroy (recommended)
  2. Destroy without checkpointing
  3. Detach only (keep Sprite running, disconnect)
  4. Cancel
- Auto-checkpointing: configurable (time-based or event-based after Claude Code task completion)
- `slua checkpoint <name>` — quick checkpoint shortcut
- Dashboard: `d` key to destroy with prompt, `c` key to checkpoint

**Files to create/modify:**
- `cmd/destroy.go` — destroy command with interactive prompt
- `cmd/checkpoint.go` — checkpoint shortcut command
- `internal/sprites/checkpoint.go` — checkpoint management (create, list, auto)
- `internal/tui/confirm.go` — confirmation dialog Bubble Tea model
- `internal/tui/dashboard.go` — add `d` and `c` key bindings

**Success criteria:**
- [ ] `slua destroy` always prompts before destructive action
- [ ] Auto-checkpoint fires after configurable interval or event
- [ ] Checkpoint creation takes <1s (Sprites API is ~300ms)
- [ ] Dashboard shortcuts work for destroy + checkpoint

**Estimated scope:** ~400-600 lines of Go

---

### Phase 4: Setup Wizard + GitHub Integration

**Goal:** Spin up a new project with zero friction — name it, repo it, configure it, go.

**Deliverables:**
- `slua new` — interactive wizard:
  1. Name the Sprite
  2. Repository: clone existing / create new GitHub repo / start from checkpoint / blank
  3. If creating new repo: public/private, description
  4. Environment template (from .slua.yml or defaults)
  5. Auto-run: create Sprite, sync config (via sprite-up --sync), clone/init repo, install deps, create baseline checkpoint
- `slua new --from-template <name>` — use a saved template
- `.slua.yml` config file format for project templates
- GitHub integration: `gh repo create` wrapper, credential setup on Sprite

**Files to create/modify:**
- `cmd/new.go` — new command (wizard entry point)
- `internal/tui/wizard.go` — Bubble Tea wizard model (multi-step form)
- `internal/github/create.go` — GitHub repo creation via gh CLI
- `internal/github/sync.go` — credential setup + push verification
- `internal/setup/bootstrap.go` — orchestrates sprite-up + repo + deps
- `internal/setup/template.go` — .slua.yml parsing and application
- `config/default.slua.yml` — default project template

**The GitHub integration fills a gap in the existing sprite-workflow skill:**

The current `sprite-up.sh` can clone an existing repo but cannot:
- Create a new GitHub repo
- Configure GitHub credentials on the Sprite
- Push an initial commit
- Set up the repo from scratch

Slua's wizard wraps sprite-up AND adds these missing steps:

```
slua new
  → "Name?" → my-project
  → "Repository?" → Create new GitHub repo
  → "Public or private?" → Public
  → "Description?" → My awesome project

  Running:
  ✓ Created GitHub repo JPM1118/my-project
  ✓ Created Sprite 'my-project'
  ✓ Synced Claude Code config (skills, plugins, settings)
  ✓ Cloned repo into Sprite
  ✓ Configured GitHub credentials on Sprite
  ✓ Installed dependencies
  ✓ Created baseline checkpoint

  Ready! Connect with: slua connect my-project
```

**Success criteria:**
- [ ] `slua new` creates a fully configured Sprite with GitHub repo in one flow
- [ ] GitHub repo is accessible and pushable from inside the Sprite
- [ ] Environment is consistent across all projects (same tools, same config)
- [ ] Templates allow customization for different project types

**Estimated scope:** ~800-1200 lines of Go

---

## .slua.yml Template Format

```yaml
# ~/.config/slua/templates/default.yml
name: default
description: Standard Claude Code development environment

# Sprite configuration
sprite:
  sync: true  # Run sprite-up --sync

# GitHub defaults
github:
  visibility: public
  default_branch: main

# Auto-checkpoint settings
checkpoints:
  auto: true
  interval: 30m  # Time-based auto-checkpoint
  on_completion: true  # Checkpoint when Claude Code task finishes

# Notifications
notifications:
  desktop: true
  sound: true
  ntfy:
    enabled: false
    topic: ""  # ntfy.sh topic for phone notifications

# Post-setup commands (run inside the Sprite after bootstrap)
setup:
  - git config user.name "Your Name"
  - git config user.email "your@email.com"
```

---

## Acceptance Criteria

### Functional Requirements

- [ ] `slua dashboard` displays all active Sprites with real-time status
- [ ] `slua new` creates a Sprite with optional GitHub repo in a single interactive flow
- [ ] `slua destroy` prompts for checkpoint before any destructive action
- [ ] Desktop notifications fire when a Sprite needs user attention
- [ ] `slua cycle next/prev` switches between Sprites quickly
- [ ] Auto-checkpointing works on configurable schedule
- [ ] GitHub credentials are properly configured on new Sprites
- [ ] Works on Linux and macOS

### Non-Functional Requirements

- [ ] Dashboard refreshes within 5s of state changes
- [ ] Checkpoint creation completes in <2s
- [ ] Handles 20+ concurrent Sprite connections without performance degradation
- [ ] Single binary distribution (Go cross-compilation)
- [ ] Config via YAML file + environment variables

### Quality Gates

- [ ] Unit tests for API client, parser, and state machine
- [ ] Integration test with a real Sprite (create → monitor → checkpoint → destroy)
- [ ] No hardcoded credentials (all via env vars or config)
- [ ] `go vet` and `golangci-lint` pass

---

## Dependencies and Prerequisites

| Dependency | Status | Notes |
|-----------|--------|-------|
| Sprites CLI (`sprite`) | Installed | Required for `sprite-up` and `sprite console` |
| Sprites API token | Required | `SPRITES_TOKEN` env var |
| GitHub CLI (`gh`) | Installed | Required for repo creation in wizard |
| Go 1.22+ | Required | Build toolchain |
| Bubble Tea | Available | github.com/charmbracelet/bubbletea |
| Lip Gloss | Available | github.com/charmbracelet/lipgloss |
| beeep | Available | github.com/gen2brain/beeep |
| gorilla/websocket | Available | WebSocket client for Go |

---

## Risk Analysis and Mitigation

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Claude Code output patterns change | State detection breaks | Medium | Make patterns configurable, not hardcoded. Fallback to polling-only. |
| Sprites API changes | Client breaks | Low | Pin SDK version, monitor changelog |
| WebSocket connections drop | Missed notifications | Medium | Auto-reconnect with exponential backoff |
| Many concurrent WebSocket connections | Resource exhaustion | Low | Cap at configurable max (default 20), queue the rest |
| `sprite console` not available | Cannot connect from dashboard | Low | Fallback to `sprite exec` with TTY |

---

## Open Questions

1. **Should `slua` manage tmux sessions on the local side?** When you "connect" from the dashboard, should it open a new terminal tab, a tmux pane, or just take over the current terminal?
2. **ntfy integration scope** — Should ntfy be a first-class feature or a plugin/extension?
3. **Multi-org support** — Should `slua` support managing Sprites across multiple Fly.io organizations?
4. **Sprite grouping** — Should Sprites be groupable by project/tag for filtering?

---

## Future Considerations

- **Sprite-to-Sprite awareness** — If one Sprite needs output from another, could Slua coordinate?
- **Cost tracking** — Show running costs per Sprite in the dashboard
- **Web dashboard** — Optional browser UI alongside the TUI
- **Plugin system** — Allow custom notification handlers, state detectors, setup steps

---

## References

### Internal
- Research findings: `docs/research.md`
- Sprite workflow skill: `~/.claude/skills/sprite-workflow/`
- sprite-up.sh: `~/.claude/skills/sprite-workflow/scripts/sprite-up.sh`

### External
- Sprites API: https://sprites.dev/api
- Sprites Design Blog: https://fly.io/blog/design-and-implementation/
- sprites-go SDK: https://github.com/superfly/sprites-go
- sprites-js SDK: https://github.com/superfly/sprites-js
- Bubble Tea: https://github.com/charmbracelet/bubbletea
- Claude Squad (reference implementation): https://github.com/smtg-ai/claude-squad
- Agent Deck (reference implementation): https://github.com/asheshgoplani/agent-deck
- ntfy: https://ntfy.sh/
- beeep: https://github.com/gen2brain/beeep

---

*Plan created 2026-02-05*
