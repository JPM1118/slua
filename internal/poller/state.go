package poller

import (
	"time"
)

const (
	// MaxBackoff is the maximum interval between polls for failing Sprites.
	MaxBackoff = 5 * time.Minute

	// UnreachableThreshold is the number of consecutive failures before
	// a Sprite is marked UNREACHABLE.
	UnreachableThreshold = 3
)

// SpriteState tracks the polled state of a single Sprite.
type SpriteState struct {
	Name           string
	Status         string
	PreviousStatus string
	LastPollTime   time.Time
	ConsecFails    int
	BackoffUntil   time.Time
	ErrorDetail    string // exit code for ERROR state
}

// ShouldPoll returns true if this Sprite is ready to be polled.
func (s *SpriteState) ShouldPoll(now time.Time) bool {
	return now.After(s.BackoffUntil) || now.Equal(s.BackoffUntil)
}

// RecordSuccess records a successful poll result.
// Returns true if the status changed (a transition occurred).
func (s *SpriteState) RecordSuccess(status string, now time.Time) bool {
	s.PreviousStatus = s.Status
	s.Status = status
	s.LastPollTime = now
	s.ConsecFails = 0
	s.BackoffUntil = time.Time{}
	return s.IsTransition()
}

// RecordFailure records a failed poll attempt and calculates backoff.
func (s *SpriteState) RecordFailure(baseInterval time.Duration, now time.Time) {
	s.ConsecFails++
	s.LastPollTime = now

	// Calculate exponential backoff: base * 2^(fails-1), capped at MaxBackoff
	backoff := baseInterval
	for i := 1; i < s.ConsecFails; i++ {
		backoff *= 2
		if backoff > MaxBackoff {
			backoff = MaxBackoff
			break
		}
	}
	s.BackoffUntil = now.Add(backoff)

	// After threshold, mark as UNREACHABLE
	if s.ConsecFails >= UnreachableThreshold {
		s.PreviousStatus = s.Status
		s.Status = "UNREACHABLE"
	}
}

// IsTransition returns true if the current status differs from the previous.
func (s *SpriteState) IsTransition() bool {
	return s.PreviousStatus != "" && s.PreviousStatus != s.Status
}
