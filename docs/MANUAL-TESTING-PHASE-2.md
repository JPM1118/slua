# Phase 2 Manual Testing Guide — State Polling + Notifications

Manual verification checklist for PR #2. Covers polling, notifications, bell, config, and edge cases.

## Prerequisites

```bash
go build -o slua .
sprite list          # need at least 1 running Sprite; 2+ ideal
```

---

## Test 1: Dashboard Launches with Polling

```bash
./slua dashboard
```

- [ ] Dashboard renders with header "Slua Si"
- [ ] All Sprites appear with correct names, statuses, uptimes
- [ ] Subheader shows "Connected" (or briefly "Loading...")
- [ ] After ~15s, subheader updates to show "Last poll: Xs ago"

---

## Test 2: State Detection & Transitions

**Setup:** Have a Sprite running Claude Code. In a separate terminal:

```bash
sprite console -s <sprite-name>
# Start a Claude Code session that will pause for user input
```

**In the dashboard:**

- [ ] Sprite shows WORKING while Claude Code is actively generating
- [ ] When Claude Code pauses for input (Y/n prompt), status changes to WAITING within ~15-30s
- [ ] Notification bar shows transition: `<name>: WORKING -> WAITING`
- [ ] At width >= 100, LAST ACTIVITY column shows "needs input" for WAITING sprites

---

## Test 3: Terminal Bell

**Setup:** Ensure your terminal supports audible/visual bell (iTerm2: Preferences > Profiles > Terminal > "Audible bell" or "Visual bell").

```bash
./slua dashboard
```

- [ ] When a Sprite transitions to WAITING, you hear/see a bell
- [ ] Bell does NOT ring again within 30s (debounce)
- [ ] After 30s, a new transition rings the bell again
- [ ] Attention badge appears top-right: `[N need attention]`

---

## Test 4: Manual Refresh (r key)

```bash
./slua dashboard
# Press 'r'
```

- [ ] Subheader briefly shows "Loading..."
- [ ] Sprite list refreshes (new Sprites appear, removed ones disappear)
- [ ] "Last poll" timestamp resets to a few seconds ago
- [ ] Works even if previous poll was recent

---

## Test 5: Shell-Out and Bell Suspend/Resume

```bash
./slua dashboard
# Navigate to a WAITING sprite with j/k
# Press Enter
```

- [ ] Terminal switches to Sprite console (Claude Code visible)
- [ ] While in console, no bells ring even if other Sprites change state
- [ ] Notification for the connected Sprite is cleared from bar
- [ ] After exiting console (Ctrl+C or exit), dashboard resumes
- [ ] Sprite list refreshes immediately on return
- [ ] If attention states still exist, bell rings once after return
- [ ] "Last poll" timestamp is fresh after return

---

## Test 6: Notification Bar Behavior

```bash
./slua dashboard
# Wait for multiple state transitions
```

- [ ] Notification bar appears above the status bar
- [ ] Shows most recent transition with sprite name and new status
- [ ] If 2+ transitions happen, only 2 most recent are visible
- [ ] Pressing Enter on a sprite with a notification clears that notification
- [ ] Long sprite names are truncated with ellipsis if terminal is narrow

---

## Test 7: Error Handling

```bash
./slua dashboard
```

- [ ] If `sprite list` fails, error message appears in notification bar
- [ ] If a Sprite becomes unreachable (3 consecutive poll failures), status changes to UNREACHABLE
- [ ] Refreshing with `r` retries the failed operation
- [ ] If refresh succeeds, error clears

---

## Test 8: Config File

```bash
mkdir -p ~/.config/slua
cat > ~/.config/slua/config.yml << 'EOF'
detection:
  poll_interval: 5s
  exec_timeout: 3s
  prompt_patterns:
    - "Y/n"
    - "Continue?"
notifications:
  terminal_bell: true
  bell_debounce: 10s
  bell_on_states:
    - WAITING
    - ERROR
EOF

./slua dashboard
```

- [ ] Polling happens every ~5s instead of default 15s (watch "Last poll" updates)
- [ ] Bell debounce is shorter (10s instead of 30s)
- [ ] Dashboard launches without config warnings in stderr

```bash
rm ~/.config/slua/config.yml
./slua dashboard
```

- [ ] Dashboard launches normally with defaults
- [ ] Polling at ~15s interval

---

## Test 9: Responsive Layout

```bash
./slua dashboard
# Resize terminal window
```

- [ ] Width >= 100: LAST ACTIVITY column visible ("active", "idle", "needs input")
- [ ] Width < 100: LAST ACTIVITY column hidden, other columns readable
- [ ] Width < 80 or height < 24: "Terminal too small" message
- [ ] Resizing back restores normal view

---

## Test 10: Navigation Edge Cases

```bash
./slua dashboard
```

- [ ] `j`/`k` navigate up/down, cursor clamps at boundaries
- [ ] `G` jumps to last sprite, `g` jumps to first
- [ ] `q` exits cleanly (no crash, no hung processes)
- [ ] Ctrl+C also exits cleanly
- [ ] With 0 Sprites running: "No Sprites running" message, Enter does nothing

---

## Test 11: Concurrent Polling Under Load

```bash
# With 5+ running Sprites:
./slua dashboard
```

- [ ] All running Sprites get polled (statuses update)
- [ ] Non-running Sprites (SLEEPING, CREATING) are skipped
- [ ] Dashboard remains responsive during polling
- [ ] No visual glitches or stuttering
