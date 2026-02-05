---
title: "Phase 1: TUI Dashboard with Shell-Out Orchestrator Pattern"
date: 2026-02-05
category: architecture
tags: [tui, go, bubble-tea, lip-gloss, cobra, shell-out, sprite, fly-io, interface-design, testability]
module: slua-si
symptoms:
  - Manual sprite CLI invocation required for each instance
  - No centralized visibility into multiple Sprite instance states
  - No CLI parity with TUI actions (agent-native gap)
  - Magic strings scattered across multiple files
severity: high
---

# Phase 1: TUI Dashboard with Shell-Out Orchestrator Pattern

## Problem

Teams operating multiple Fly.io Sprite instances running Claude Code had no unified control plane. Each operation required manual `sprite` CLI commands across multiple terminal windows — no single-pane-of-glass for status, no quick console access, no notification routing. A "control tower" dashboard was needed that preserves `sprite login` authentication without re-implementing the SDK.

## Root Cause

The `sprite` CLI provides individual commands but no aggregated view. The Go SDK exists but is immature and would require separate auth management. The gap is an orchestration layer that composes existing CLI primitives into a cohesive TUI.

## Solution

### Architecture: Shell-Out Orchestrator

```
cmd/ (Cobra CLI) → internal/tui/ (Bubble Tea) → internal/sprites/ (CLI wrapper)
```

The TUI suspends itself via `tea.ExecProcess` to hand off interactive sessions to `sprite console -s <name>`. This avoids terminal emulator complexity and inherits authentication from `sprite login`.

### Key Decisions

1. **CLI over SDK** — All Sprite operations go through the `sprite` binary. Inherits auth, handles edge cases, avoids SDK churn.

2. **SpriteSource interface** — Strategy pattern decoupling TUI from concrete CLI:
   ```go
   type SpriteSource interface {
       List(ctx context.Context) ([]Sprite, error)
       ConsoleCmd(name string) *exec.Cmd
   }
   ```
   Enables mock-based testing without requiring the `sprite` binary.

3. **Context with timeout** — 10s `context.WithTimeout` on `exec.CommandContext` prevents indefinite hangs when the sprite API is unreachable.

4. **Exported status constants** — Replaced ~35 magic string occurrences across 4 files with typed constants (`StatusWorking`, `StatusSleeping`, etc.).

5. **Pre-allocated styles** — Lip Gloss styles stored as package-level vars to avoid N allocations per render cycle.

6. **Agent-native CLI parity** — Added `slua connect <name>` and `slua status --json` so every TUI action has a non-interactive equivalent.

### Review Process

6 parallel review agents ran post-implementation:
- **Security**: No injection risks (exec.Command with argv, not `sh -c`). Added `.gitignore` credential patterns.
- **Architecture**: Clean dependency graph, proper interface boundaries, ready for Phase 2.
- **Patterns**: Identified magic string problem, missing context support — both fixed.
- **Performance**: Pre-allocated styles, identified future optimizations (rune-width, attention caching).
- **Simplicity**: Low complexity score, no over-engineering detected.
- **Agent-native**: Identified missing `connect` command and `--json` flag — both added.

## What Worked

- **Interface-first design**: Defining `SpriteSource` before implementation enabled 15 TUI unit tests with zero external dependencies.
- **Elm architecture (Bubble Tea)**: Clean separation of Init/Update/View made testing straightforward — inject mock data, call Update with key messages, assert on View output.
- **Parallel review agents**: Running 6 agents simultaneously caught issues across different dimensions that a single review would miss.

## What to Watch For

- **`padRight`/`truncate` use byte length, not rune width** — Will misalign columns with non-ASCII sprite names. Use `lipgloss.Width()` instead of `len()`. (P3, deferred to Phase 2)
- **Attention count scanned per render** — O(n) scan in `renderHeader()` on every View call. Cache on data load for O(1) per render. (P3)
- **Status switch statements in 4 places** — Adding a new status requires changes in `normalizeStatus`, `statusStyle`, `statusLabel`, and `activityText`. Consider a status metadata registry for Phase 2.

## Prevention Strategies

1. **Define abstraction boundaries (interfaces) before implementation** — Prevents coupling to external tools and enables parallel testing.
2. **Use string constants for domain enumerations from day one** — Retroactive refactoring across multiple files is costly.
3. **Always use `context.WithTimeout` on subprocess/network calls** — A hung process is the worst kind of performance problem.
4. **Ensure CLI parity with every TUI action** — Agent-native review catches gaps that human review misses.

## Files

| File | Lines | Purpose |
|------|-------|---------|
| `internal/tui/dashboard.go` | 374 | Bubble Tea model (state machine) |
| `internal/tui/dashboard_test.go` | 334 | 15 TUI unit tests |
| `internal/sprites/cli.go` | 191 | CLI wrapper, JSON parser, status constants |
| `internal/sprites/cli_test.go` | 146 | 7 parser/formatter tests |
| `internal/sprites/source.go` | 13 | SpriteSource interface |
| `internal/tui/styles.go` | 90 | Pre-allocated Lip Gloss styles |
| `cmd/connect.go` | 26 | Agent-native console command |
| `cmd/status.go` | 49 | Non-interactive status with --json |
| `cmd/dashboard.go` | 39 | Dashboard entry point |
| `cmd/root.go` | 40 | Cobra root, defaults to dashboard |

## Cross-References

- Implementation plan: `docs/plans/2026-02-05-feat-slua-si-tui-orchestrator-plan.md`
- Brainstorm: `docs/brainstorms/2026-02-05-slua-si-tui-orchestrator-brainstorm.md`
- Research: `docs/research.md`
- Project conventions: `CLAUDE.md`
- PR: https://github.com/JPM1118/slua/pull/1
