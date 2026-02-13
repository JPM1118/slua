---
title: "feat: Phase 2 — State Polling + Notifications"
type: feat
date: 2026-02-05
---

# Phase 2: State Polling + Notifications

**Goal:** Know when a Sprite needs attention without manually checking.

**Branch:** `feat/phase-2-polling-notifications`
**Estimated scope:** ~600-800 lines of Go (6 new files, 4 modified)
**Parent plan:** `docs/plans/2026-02-05-feat-slua-si-tui-orchestrator-plan.md`

---

## Overview

Phase 1 displays Sprite status from the `sprite api /sprites` endpoint (machine-level: running/stopped). Phase 2 adds **application-level state detection** by polling each Sprite via `sprite exec` every 15 seconds to detect what Claude Code is actually doing (working, waiting for input, finished, errored). State changes trigger visual updates, notification bar entries, and terminal bells.

---

## Architecture Decisions

### AD1: Poller-to-Dashboard Communication

The poller runs as a background goroutine. It communicates with the Bubble Tea model via a **channel-based subscription command**.

Pattern:
1. `Dashboard.Init()` returns a `tea.Cmd` that launches the poller and blocks on a `chan pollerUpdateMsg`.
2. The poller writes `pollerUpdateMsg` to the channel on every state change.
3. The `Update()` handler processes the message, updates state, and returns a new subscription command to continue listening.

This is idiomatic Bubble Tea — no `tea.Program` reference needed. The poller owns the channel and the dashboard subscribes to it.

```go
// In Dashboard.Init():
return tea.Batch(d.loadSprites(), d.subscribeToPoller())

// subscribeToPoller returns a tea.Cmd that blocks on the poller's output channel
func (d Dashboard) subscribeToPoller() tea.Cmd {
    return func() tea.Msg {
        return <-d.pollerCh  // blocks until poller sends an update
    }
}

// In Update(), after handling pollerUpdateMsg:
return d, d.subscribeToPoller()  // re-subscribe for next update
```

### AD2: API vs. Poller State Precedence

Two status sources exist: the `sprite api /sprites` endpoint (machine-level) and the poller's `sprite exec` detection (application-level). Rules:

| API Status | Poller Output | Displayed Status | Poller Active? |
|---|---|---|---|
| running | WORKING | WORKING | Yes |
| running | WAITING | WAITING | Yes |
| running | FINISHED | FINISHED | Yes |
| running | ERROR:N | ERROR | Yes |
| running | (timeout) | UNREACHABLE | Yes (backoff) |
| running | (unparseable) | SLEEPING | Yes |
| stopped/suspended | (not polled) | SLEEPING | No |
| creating | (not polled) | CREATING | No |
| destroying | (not polled) | DESTROYING | No |

**Rule:** API lifecycle states (CREATING, DESTROYING, SLEEPING/stopped) take precedence. The poller only operates on Sprites whose API status is "running"/"started". On each poll cycle, the poller calls `List()` to discover current Sprites and filters to running ones.

### AD3: Parallel Polling with Bounded Concurrency

Sprites are polled **in parallel** with a goroutine pool capped at **10 concurrent** `sprite exec` calls. Each call has a 5-second timeout. With 10 workers and 5s timeout, 50 Sprites complete in ~25s (worst case), keeping within the 15s target for most realistic deployments (1-20 Sprites).

### AD4: Bell Behavior

- **Global debounce:** Max 1 bell per 30 seconds across all Sprites.
- **During shell-out:** Bells are **discarded** (not queued). On return from console, if any Sprites still need attention, a single bell rings.
- **Triggers:** WAITING and ERROR transitions only (configurable via `bell_on_states`).

### AD5: Notification Lifecycle

- Notifications persist until displaced by newer ones (FIFO, max 2 visible).
- Up to 20 notifications buffered during shell-out; only most recent 2 shown on resume.
- No auto-dismiss timeout.
- Format: `"<name>: <old> → <new> (<relative time>)"`, truncated to fit bar width.
- "Connecting to a WAITING Sprite clears its notification" means: on `Enter` press, remove that Sprite's notifications from the bar and decrement the attention badge. If still WAITING on next poll, notification and badge reappear.

### AD6: Backoff Parameters

- Base interval: poll interval (15s default)
- Multiplier: 2x per consecutive failure
- Maximum: 5 minutes
- Reset: on first successful poll
- UNREACHABLE Sprites continue to be polled at the backed-off interval
- Per-Sprite backoff (each Sprite tracks its own failure count)

### AD7: Config Validation

- `poll_interval`: minimum 5s, maximum 5m. Default 15s.
- `exec_timeout`: minimum 2s, maximum equal to `poll_interval`. Default 5s.
- `prompt_patterns`: validated as valid regex at startup. Invalid patterns log a warning and are skipped.
- Missing config file: use defaults silently.
- Malformed YAML: log error, use defaults.
- Respect `$XDG_CONFIG_HOME` (default `~/.config`).

---

## Deliverables

### New Files

#### `internal/poller/poller.go` — Background poll loop

Manages the poll lifecycle: starts/stops goroutine pool, maintains per-Sprite state, sends updates to the dashboard via channel.

```go
type Poller struct {
    cli        sprites.SpriteSource
    cfg        Config
    states     map[string]*SpriteState  // keyed by Sprite name
    updateCh   chan PollerUpdate
    stopCh     chan struct{}
    mu         sync.RWMutex
}

type PollerUpdate struct {
    States  map[string]SpriteState
    Err     error
}

type Config struct {
    PollInterval   time.Duration
    ExecTimeout    time.Duration
    PromptPatterns []string
    MaxWorkers     int  // default 10
}
```

- `New(cli SpriteSource, cfg Config) *Poller` — creates poller with output channel
- `Start(ctx context.Context)` — launches background goroutine
- `Updates() <-chan PollerUpdate` — returns read-only channel for dashboard subscription
- `Stop()` — signals shutdown, waits for in-flight polls to complete
- `TriggerNow()` — forces an immediate poll cycle (for `r` key)

Poll loop:
1. Call `cli.List(ctx)` to discover running Sprites
2. Filter to API status "running"/"started"
3. Fan out `sprite exec` detection calls to worker pool
4. Collect results, compare to previous state, emit `PollerUpdate` with changes
5. Sleep for `poll_interval`, repeat

#### `internal/poller/detector.go` — Status detection logic

Runs the detection command on a single Sprite and parses the output.

```go
func Detect(ctx context.Context, cli sprites.SpriteSource, name string, patterns []string) (string, error)
```

- Calls `cli.ExecStatus(ctx, name)` with the detection shell script
- Parses output: `WORKING`, `WAITING`, `FINISHED`, `ERROR:<code>`, or unparseable → `SLEEPING`
- Timeout → returns error (caller marks UNREACHABLE)
- The detection script is constructed from configurable `prompt_patterns`

Detection script (built dynamically from config patterns):
```bash
if pgrep -a claude > /dev/null 2>&1; then
  RECENT=$(tmux capture-pane -p -l 5 2>/dev/null || echo "")
  if echo "$RECENT" | grep -qE "(Y/n|y/N|...configured patterns...)"; then
    echo "WAITING"
  else
    echo "WORKING"
  fi
else
  EXIT=$(tmux show-environment CLAUDE_EXIT 2>/dev/null | cut -d= -f2 || echo "")
  if [ "$EXIT" = "0" ] || [ -z "$EXIT" ]; then
    echo "FINISHED"
  else
    echo "ERROR:$EXIT"
  fi
fi
```

#### `internal/poller/state.go` — Per-Sprite state tracking

```go
type SpriteState struct {
    Name           string
    Status         string    // StatusWorking, StatusWaiting, etc.
    PreviousStatus string    // for transition detection
    LastPollTime   time.Time
    ConsecFails    int       // for backoff calculation
    BackoffUntil   time.Time // skip polling until this time
    ErrorDetail    string    // exit code for ERROR state
}
```

- `(s *SpriteState) ShouldPoll(now time.Time) bool` — checks backoff
- `(s *SpriteState) RecordSuccess(status string)` — resets backoff, detects transition
- `(s *SpriteState) RecordFailure()` — increments failure count, calculates next backoff
- `(s *SpriteState) IsTransition() bool` — true if status changed from previous

#### `internal/notify/terminal.go` — Terminal bell with debounce

```go
type Bell struct {
    debounce   time.Duration
    lastRing   time.Time
    suspended  bool  // true during shell-out
    triggerOn  map[string]bool  // states that trigger bell
}
```

- `New(debounce time.Duration, states []string) *Bell`
- `Ring(status string) bool` — rings bell if debounce allows and not suspended, returns whether it rang
- `Suspend()` / `Resume() bool` — resume returns true if attention states exist (ring single bell)

#### `internal/notify/bar.go` — Notification bar FIFO queue

```go
type Notification struct {
    SpriteName string
    OldStatus  string
    NewStatus  string
    Timestamp  time.Time
}

type Bar struct {
    items    []Notification
    maxStore int  // max buffered (default 20)
}
```

- `Push(n Notification)` — appends, trims to maxStore
- `Visible() []Notification` — returns last 2
- `ClearForSprite(name string)` — removes notifications for a specific Sprite
- `Render(width int) string` — formats visible notifications for display

#### `internal/config/config.go` — Config file parsing

```go
type Config struct {
    Detection    DetectionConfig    `yaml:"detection"`
    Notifications NotificationConfig `yaml:"notifications"`
}

type DetectionConfig struct {
    PollInterval   duration `yaml:"poll_interval"`
    ExecTimeout    duration `yaml:"exec_timeout"`
    PromptPatterns []string `yaml:"prompt_patterns"`
}

type NotificationConfig struct {
    TerminalBell bool     `yaml:"terminal_bell"`
    BellDebounce duration `yaml:"bell_debounce"`
    BellOnStates []string `yaml:"bell_on_states"`
}
```

- `Load() (*Config, error)` — reads from `$XDG_CONFIG_HOME/slua/config.yml` (default `~/.config/slua/config.yml`), merges with defaults
- `Defaults() *Config` — returns sensible defaults
- Custom `duration` type that unmarshals YAML strings like `"15s"` to `time.Duration`
- Validates bounds (poll_interval 5s-5m, exec_timeout 2s-poll_interval)
- Invalid patterns logged and skipped, not fatal

### Modified Files

#### `internal/sprites/source.go` — Add ExecStatus method

```go
type SpriteSource interface {
    List(ctx context.Context) ([]Sprite, error)
    ConsoleCmd(name string) *exec.Cmd
    ExecStatus(ctx context.Context, name string, script string) (string, error)  // NEW
}
```

#### `internal/sprites/cli.go` — Implement ExecStatus

```go
func (c *CLI) ExecStatus(ctx context.Context, name string, script string) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    cmd := c.spriteCmd(ctx, "exec", "-s", name, "--", "sh", "-c", script)
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    if err := cmd.Run(); err != nil {
        errMsg := strings.TrimSpace(stderr.String())
        if errMsg == "" {
            errMsg = err.Error()
        }
        return "", fmt.Errorf("sprite exec %s: %s", name, errMsg)
    }
    return strings.TrimSpace(stdout.String()), nil
}
```

#### `internal/tui/dashboard.go` — Poller integration

New fields on `Dashboard`:
```go
type Dashboard struct {
    // ... existing fields ...
    poller      *poller.Poller
    pollerCh    <-chan poller.PollerUpdate
    bell        *notify.Bell
    notifyBar   *notify.Bar
    suspended   bool   // true during shell-out
    lastPoll    time.Time
}
```

New message type:
```go
type pollerUpdateMsg struct {
    states map[string]poller.SpriteState
    err    error
}
```

Changes to `Update()`:
- Handle `pollerUpdateMsg`: merge polled states into `d.sprites`, update notification bar, ring bell if applicable, re-subscribe to poller channel
- Handle `consoleFinishedMsg`: set `d.suspended = false`, call `d.bell.Resume()`, trigger immediate poll
- Before `tea.ExecProcess`: set `d.suspended = true`, call `d.bell.Suspend()`
- On `"enter"` with WAITING Sprite: call `d.notifyBar.ClearForSprite(name)`
- On `"r"` key: call `d.poller.TriggerNow()` in addition to `loadSprites()`

Changes to `View()`:
- `renderSubheader()`: show `"Last poll: Xs ago"` when connected
- `renderNotificationBar()`: delegate to `d.notifyBar.Render(d.width)` instead of just showing `d.lastErr`

#### `cmd/dashboard.go` — Wire up config and poller

```go
func runDashboard() error {
    cfg, err := config.Load()
    // ... handle error ...

    cli := &sprites.CLI{Org: org}

    p := poller.New(cli, poller.Config{
        PollInterval:   cfg.Detection.PollInterval,
        ExecTimeout:    cfg.Detection.ExecTimeout,
        PromptPatterns: cfg.Detection.PromptPatterns,
        MaxWorkers:     10,
    })

    bell := notify.NewBell(cfg.Notifications.BellDebounce, cfg.Notifications.BellOnStates)
    bar := notify.NewBar(20)  // max 20 buffered

    model := tui.NewDashboard(cli, tui.WithPoller(p), tui.WithBell(bell), tui.WithNotifyBar(bar))
    program := tea.NewProgram(model, tea.WithAltScreen())

    // Start poller before Run() — messages queue until program starts processing
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    p.Start(ctx)
    defer p.Stop()

    finalModel, err := program.Run()
    // ...
}
```

---

## Implementation Order

### Step 1: Config parsing (`internal/config/config.go`)
- [x] Create `internal/config/` directory
- [x] Add `gopkg.in/yaml.v3` dependency
- [x] Implement `Config` struct with defaults
- [x] Implement `Load()` with XDG support
- [x] Custom `duration` YAML type
- [x] Bounds validation
- [x] Tests: missing file, valid file, partial file, invalid values, XDG override

### Step 2: State tracking (`internal/poller/state.go`)
- [x] Define `SpriteState` struct
- [x] Implement `ShouldPoll`, `RecordSuccess`, `RecordFailure`
- [x] Implement backoff calculation (2x, max 5m, reset on success)
- [x] Implement transition detection
- [x] Tests: backoff progression, reset, transition detection, concurrent failure threshold

### Step 3: Detector (`internal/poller/detector.go`)
- [x] Implement `Detect()` function
- [x] Build detection script from configurable patterns
- [x] Parse output: WORKING, WAITING, FINISHED, ERROR:N, unparseable → SLEEPING
- [x] Tests: all output variants, timeout handling, script construction with custom patterns

### Step 4: SpriteSource extension (`internal/sprites/source.go`, `internal/sprites/cli.go`)
- [x] Add `ExecStatus(ctx, name, script) (string, error)` to interface
- [x] Implement on `CLI` struct
- [x] Update interface compliance check
- [x] Update mock in `dashboard_test.go`
- [x] Tests: successful exec, timeout, stderr error

### Step 5: Notification bar (`internal/notify/bar.go`)
- [x] Implement `Bar` struct with FIFO queue
- [x] `Push`, `Visible`, `ClearForSprite`, `Render`
- [x] Tests: FIFO ordering, max buffer, clear by sprite, render formatting

### Step 6: Terminal bell (`internal/notify/terminal.go`)
- [x] Implement `Bell` struct with debounce
- [x] `Ring`, `Suspend`, `Resume`
- [x] Tests: debounce timing, suspend/resume, state filtering

### Step 7: Poller (`internal/poller/poller.go`)
- [x] Implement `Poller` struct with worker pool
- [x] `New`, `Start`, `Stop`, `Updates`, `TriggerNow`
- [x] Poll loop: List → filter running → fan out detect → collect → emit update
- [x] Per-Sprite backoff integration
- [x] Tests: mock SpriteSource, verify state transitions, verify channel output, verify stop/cleanup

### Step 8: Dashboard integration (`internal/tui/dashboard.go`)
- [x] Add new fields (poller, bell, notifyBar, suspended, lastPoll)
- [x] Add `pollerUpdateMsg` type
- [x] Implement channel subscription pattern in `Init()`
- [x] Handle `pollerUpdateMsg` in `Update()`: merge states, update notifications, ring bell
- [x] Update shell-out flow: set suspended, clear on return
- [x] Update `renderSubheader()` with last-poll timestamp
- [x] Update `renderNotificationBar()` to use `Bar.Render()`
- [x] Clear notification on Enter for WAITING Sprite
- [x] `r` key triggers `poller.TriggerNow()`
- [x] Tests: mock poller updates, bell suppression during shell-out, notification rendering

### Step 9: Wiring (`cmd/dashboard.go`)
- [x] Load config
- [x] Create poller, bell, bar
- [x] Pass to `NewDashboard` via options
- [x] Start/stop poller lifecycle
- [x] Tests: verify startup/shutdown flow

### Step 10: Verification
- [x] `go test ./...` — all tests pass
- [x] `go build -o slua .` — compiles
- [x] `go vet ./...` — clean
- [ ] Manual test with real Sprites (if available)

---

## Acceptance Criteria

- [ ] Dashboard shows live state (WORKING/WAITING/FINISHED/ERROR/UNREACHABLE) with color
- [ ] State updates within 15s of actual change
- [ ] Terminal bell on WAITING/ERROR transitions (debounced, max 1 per 30s globally)
- [ ] No bells ring while inside `sprite console`
- [ ] Notification bar shows last 2 state changes with timestamps
- [ ] Badge count in header reflects Sprites needing attention
- [ ] Connecting to a WAITING Sprite clears its notification
- [ ] Poll interval configurable via config file
- [ ] Missing config file uses sensible defaults
- [ ] Individual Sprite failure does not crash dashboard or affect other Sprites
- [ ] UNREACHABLE Sprites continue to be probed at backed-off intervals
- [ ] `r` key triggers immediate poll cycle
- [ ] Poller shuts down cleanly on `q`/`Ctrl+C`

---

## Test Strategy

Each new package gets its own `_test.go` file. All tests use plain Go `testing` (no testify/gomock). Table-driven tests for state machines and parsers. Mock `SpriteSource` for poller and dashboard tests.

| Package | Test Focus | Key Cases |
|---|---|---|
| `config` | YAML parsing, defaults, validation | Missing file, partial config, invalid bounds, XDG |
| `poller/state` | Backoff, transitions | Consecutive failures, reset, all state transitions |
| `poller/detector` | Script output parsing | All 5 outputs, timeout, custom patterns |
| `poller/poller` | Poll loop, concurrency | Mock source, state change emission, stop/cleanup |
| `notify/bar` | FIFO queue, rendering | Push/visible/clear, overflow, format |
| `notify/terminal` | Bell debounce, suspend | Timing, suspend/resume, state filter |
| `tui/dashboard` | Integration | Poller messages, bell suppression, notification rendering |

---

## Dependencies

New:
- `gopkg.in/yaml.v3` — config file parsing

Existing (unchanged):
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss`
- `github.com/spf13/cobra`

---

## Risks

| Risk | Mitigation |
|---|---|
| Detection script assumptions wrong (tmux structure, pgrep availability) | Test on real Sprite before finalizing. Conservative fallback to SLEEPING. |
| Prompt pattern false positives ("Permission" in code output) | Patterns configurable. Document that users should tune patterns. |
| Too many concurrent `sprite exec` calls | Bounded pool (max 10). Backoff for failing Sprites. |
| Channel backup during long shell-out | Bounded buffer on poller channel. Drop oldest if full. |
| Config file injection via prompt_patterns | Patterns passed as grep arguments, not interpolated into shell strings. |

---

## Open Questions (Deferred to Implementation)

1. **Exact tmux session/pane structure on Sprites** — needs prototyping on a real Sprite.
2. **`CLAUDE_EXIT` environment variable** — needs verification that Sprites set this.
3. **`pgrep` availability on Sprite images** — if missing, detection falls back to SLEEPING.

---

## References

- Parent plan: `docs/plans/2026-02-05-feat-slua-si-tui-orchestrator-plan.md` (Phase 2 section, lines 334-375)
- Phase 1 solution: `docs/solutions/architecture/phase-1-tui-dashboard-shell-out-orchestrator.md`
- Brainstorm: `docs/brainstorms/2026-02-05-slua-si-tui-orchestrator-brainstorm.md`
- CLAUDE.md project conventions
