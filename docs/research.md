# Slua Sí — Research Findings

**Slua Sí** (Irish: "the fairy host")
A TUI orchestrator for managing many running Sprites at once: start/stop, status, logs, grouping, and lifecycle control.

> Slua Sí: a host that gathers, moves, and endures.

---

## Problem Statement

When running 10+ Sprite instances (Fly.io microVMs) each with Claude Code, there's no way to:
- See all active sessions at a glance
- Get notified when a Sprite needs attention or finishes a task
- Cycle/switch between sessions quickly
- Auto-checkpoint or safely close sessions
- Spin up new Sprites with consistent environment setup

## Existing Tools (Not a Fit)

Researched tools designed for managing multiple AI coding agent sessions:

| Tool | Why it doesn't fit |
|------|-------------------|
| Claude Squad | Manages LOCAL tmux sessions + local git worktrees. Assumes agent runs locally. |
| Agent of Empires | Same — local tmux, local git worktrees, local agent processes. |
| Agent Deck | Go + Bubble Tea TUI, but again assumes local agent execution. |
| claude-tmux | tmux popup for local Claude Code sessions. |
| claunch | Project-based local session manager. |

**Why none of these work:** Sprites are remote Firecracker microVMs. The git repo, Claude Code context, everything lives ON the Sprite. These tools all assume local execution with local git worktrees.

## Sprite Architecture

**What Sprites are:** Firecracker microVMs on Fly.io — full Linux VMs (not containers), 100GB persistent NVMe-backed storage (JuiceFS + S3 object storage + SQLite metadata via Litestream), ~1-2s creation time, auto-sleep when inactive.

**Key architectural detail:** User code runs in an inner container while the root namespace hosts orchestration services (storage management, service registration, logging, network socket binding).

## Sprites API — The Communication Layer

The Sprites API provides full bidirectional communication without SSH or sidecars.

### API Endpoints

**Sprite Management:**
- POST /v1/sprites — Create sprite
- GET /v1/sprites — List sprites
- GET /v1/sprites/{name} — Get sprite details
- PUT /v1/sprites/{name} — Update sprite
- DELETE /v1/sprites/{name} — Destroy sprite

**Command Execution (Exec):**
- WSS /v1/sprites/{name}/exec — Execute commands via WebSocket (streaming)
- POST /v1/sprites/{name}/exec — Execute command via HTTP with stdin
- GET /v1/sprites/{name}/exec/sessions — List exec sessions
- WSS /v1/sprites/{name}/exec/{session-id} — Attach to existing session
- DELETE /v1/sprites/{name}/exec/{session-id} — Kill exec session

**Checkpoints:**
- POST /v1/sprites/{name}/checkpoints — Create checkpoint
- GET /v1/sprites/{name}/checkpoints — List checkpoints
- GET /v1/sprites/{name}/checkpoints/{id} — Get checkpoint
- POST /v1/sprites/{name}/checkpoints/{id}/restore — Restore

**Networking:**
- WSS /v1/sprites/{name}/proxy — Tunnel TCP to sprite ports
- GET /v1/sprites/{name}/policies — Get network policy
- POST /v1/sprites/{name}/policies — Set DNS-based filtering

**Services:**
- GET /v1/sprites/{name}/services — List services
- POST /v1/sprites/{name}/services — Create service
- GET /v1/sprites/{name}/services/{id}/logs — Get service logs
- POST /v1/sprites/{name}/services/{id}/start — Start service
- POST /v1/sprites/{name}/services/{id}/stop — Stop service

### SDKs

- JavaScript SDK (superfly/sprites-js) — spawn() with event streaming, exec(), sessions, TTY, port notifications
- Python SDK (superfly/sprites-py)
- Go SDK (superfly/sprites-go)

### Key SDK Capabilities (JS)

**Event-based streaming:**
```js
const cmd = sprite.spawn('ls', ['-la']);
cmd.stdout.on('data', (chunk) => { /* handle data */ });
cmd.on('exit', (code) => { /* handle completion */ });
```

**Detachable sessions:**
```js
const session = await sprite.createSession({ cmd: 'claude' });
// Later, from anywhere:
const attached = await sprite.attachSession(sessionId);
```

**Port notifications:**
```js
cmd.on('message', ({ type, port, pid }) => {
  // type: 'port_opened'
});
```

## Bidirectional Communication Architecture

**No sidecar needed.** The Sprites SDK IS the communication channel.

### Sprite to Local (monitoring/notifications)
- spawn() with WebSocket streaming of stdout/stderr — persistent connection watching Claude Code output
- createSession() / attachSession() — detachable background sessions
- message event for port notifications
- Service logs API

### Local to Sprite (commands/automation)
- exec() — send any command to a Sprite
- Checkpoint REST API — create/restore checkpoints without connecting
- Service start/stop API
- Destroy API

### Local Notification Options
- ntfy (ntfy.sh) — self-hosted push notifications to phone/desktop via HTTP POST
- dev-notify-bridge — lightweight local HTTP listener for native desktop notifications
- Terminal bell / notify-send — simplest option

## Proposed Architecture

```
Local Machine
  Slua Si (TUI)
    Sprite A: working...
    Sprite B: needs input [alert]
    Sprite C: done
    Sprite D: working...
  Communication: WebSocket + REST
        |
Fly.io
  Sprite A (microVM) — Claude Code session
  Sprite B (microVM) — Claude Code session
  Sprite C (microVM) — Claude Code session
  Sprite D (microVM) — Claude Code session
```

## Feature Layers

### Layer 1: Dashboard
- Overview of all active Sprites (name, status, uptime)
- Select to connect, kill from dashboard
- Live-updating TUI

### Layer 2: Notifications (bidirectional)
- Maintain WebSocket connections to each Sprite via SDK
- Watch Claude Code output for state patterns (waiting for input, completed, errors)
- Fire local notifications (desktop + optional ntfy for phone)
- Send commands back (auto-checkpoint, kill, etc.) without console

### Layer 3: Safe Lifecycle ("Save before close?")
- Prompt for checkpoint before destroy
- Options: checkpoint+destroy, destroy, detach, cancel
- Auto-checkpointing (time-based or event-based after Claude Code task completion)

### Layer 4: Setup Wizard
- Interactive new-Sprite creation flow
- Name, repo (clone existing or create new), branch
- Consistent environment setup (deps, tools, Claude Code config)
- Template/config file driven (.slua.yml or similar)

## Community Signals (X/Twitter)

- @alxfazio (730 likes): Building fleet of vertical agents on Sprites, each getting its own full computer
- @martin_casado (489 likes): "The sprite model really feels like the future."
- @flaviocopes (427 likes): "sprites.dev created a cloud VM in 1.6 seconds AND has Claude Code and Codex preinstalled"
- @flydotio: "Sprites can spin up and configure other Sprites"
- Sprite checkpoint differ tool: github.com/aezell/sprite-differ

---

*Research conducted 2026-02-05*
