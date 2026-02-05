# Slua Si

TUI orchestrator for managing multiple Fly.io Sprite instances running Claude Code. Control tower, not a terminal emulator — shows status, routes notifications, shells out to `sprite console` for interactive sessions.

## Commands

```bash
go build -o slua .        # build binary
go test ./...             # run all tests
go vet ./...              # static analysis
./slua                    # launch dashboard (default command)
./slua status             # non-TUI status table
./slua dashboard          # explicit dashboard launch
```

## Architecture

Shell-out orchestrator pattern: the TUI suspends itself via `tea.ExecProcess` and hands off to `sprite console -s <name>`. All Sprite operations go through the `sprite` CLI (not the Go SDK directly) to inherit authentication from `sprite login`.

- `cmd/` — Cobra commands (root defaults to dashboard)
- `internal/sprites/` — wraps `sprite` CLI, parses `sprite api` JSON responses
- `internal/tui/` — Bubble Tea models and Lip Gloss styles
- `internal/poller/` — background state polling (Phase 2)
- `internal/notify/` — notification dispatch (Phase 2)

## Key Decisions

- **Polling over WebSocket**: Background goroutine polls via `sprite exec` every 15s. No persistent WebSocket connections.
- **CLI over SDK**: Shell out to `sprite` CLI rather than using sprites-go SDK. Inherits auth, handles edge cases.
- **Modal input**: vim-style `/` to enter search mode. Single-char keybindings (`j`/`k`/`d`/`c`/`q`) active only in normal mode.
- **Auto-checkpoint on commit**: Claude Code hook creates a Sprite checkpoint after every successful git commit.

## Terminology

IMPORTANT: Avoid "task" when referring to Claude Code's work units — Claude Code uses "task" in its own interface. Use "job", "session", or "work" instead.

## Commit Style

Imperative mood, conventional format (`feat:`, `fix:`, `refactor:`). Always include `Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>`.

## Plans

Implementation plan: `docs/plans/2026-02-05-feat-slua-si-tui-orchestrator-plan.md`
Brainstorm: `docs/brainstorms/2026-02-05-slua-si-tui-orchestrator-brainstorm.md`
