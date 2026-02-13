package notify

import (
	"fmt"
	"time"
)

// Bell manages terminal bell notifications with debounce and suspension.
type Bell struct {
	debounce  time.Duration
	lastRing  time.Time
	suspended bool
	triggerOn map[string]bool
}

// NewBell creates a Bell with the given debounce interval and trigger states.
func NewBell(debounce time.Duration, states []string) *Bell {
	triggerOn := make(map[string]bool, len(states))
	for _, s := range states {
		triggerOn[s] = true
	}
	return &Bell{
		debounce:  debounce,
		triggerOn: triggerOn,
	}
}

// Ring attempts to ring the terminal bell for the given status.
// Returns true if the bell actually rang.
func (b *Bell) Ring(status string, now time.Time) bool {
	if b.suspended {
		return false
	}
	if !b.triggerOn[status] {
		return false
	}
	if now.Sub(b.lastRing) < b.debounce {
		return false
	}

	fmt.Print("\a")
	b.lastRing = now
	return true
}

// Suspend disables bell ringing (during shell-out).
func (b *Bell) Suspend() {
	b.suspended = true
}

// Resume re-enables bell ringing.
// Returns true if a bell should ring (attention states still exist).
func (b *Bell) Resume(now time.Time) bool {
	b.suspended = false
	// Caller should check if attention states exist and ring if so
	return true
}

// IsSuspended returns whether the bell is currently suspended.
func (b *Bell) IsSuspended() bool {
	return b.suspended
}

// ShouldTrigger returns whether the given status would trigger a bell
// (ignoring debounce and suspension).
func (b *Bell) ShouldTrigger(status string) bool {
	return b.triggerOn[status]
}
