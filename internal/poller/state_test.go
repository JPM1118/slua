package poller

import (
	"testing"
	"time"
)

func TestShouldPoll_NoBackoff(t *testing.T) {
	s := &SpriteState{Name: "test"}
	if !s.ShouldPoll(time.Now()) {
		t.Error("new state should be ready to poll")
	}
}

func TestShouldPoll_DuringBackoff(t *testing.T) {
	now := time.Now()
	s := &SpriteState{
		Name:         "test",
		BackoffUntil: now.Add(10 * time.Second),
	}
	if s.ShouldPoll(now) {
		t.Error("should not poll during backoff")
	}
}

func TestShouldPoll_AfterBackoff(t *testing.T) {
	now := time.Now()
	s := &SpriteState{
		Name:         "test",
		BackoffUntil: now.Add(-1 * time.Second),
	}
	if !s.ShouldPoll(now) {
		t.Error("should poll after backoff expires")
	}
}

func TestRecordSuccess_ResetsBackoff(t *testing.T) {
	now := time.Now()
	s := &SpriteState{
		Name:         "test",
		ConsecFails:  3,
		BackoffUntil: now.Add(5 * time.Minute),
	}

	s.RecordSuccess("WORKING", now)

	if s.ConsecFails != 0 {
		t.Errorf("ConsecFails = %d, want 0", s.ConsecFails)
	}
	if !s.BackoffUntil.IsZero() {
		t.Errorf("BackoffUntil should be zero, got %v", s.BackoffUntil)
	}
	if s.Status != "WORKING" {
		t.Errorf("Status = %q, want WORKING", s.Status)
	}
}

func TestRecordSuccess_DetectsTransition(t *testing.T) {
	now := time.Now()
	s := &SpriteState{Name: "test", Status: "WORKING"}

	changed := s.RecordSuccess("WAITING", now)
	if !changed {
		t.Error("WORKING → WAITING should be a transition")
	}
	if s.PreviousStatus != "WORKING" {
		t.Errorf("PreviousStatus = %q, want WORKING", s.PreviousStatus)
	}
}

func TestRecordSuccess_NoTransition(t *testing.T) {
	now := time.Now()
	s := &SpriteState{Name: "test", Status: "WORKING"}

	changed := s.RecordSuccess("WORKING", now)
	if changed {
		t.Error("WORKING → WORKING should not be a transition")
	}
}

func TestRecordSuccess_FirstPollNotTransition(t *testing.T) {
	now := time.Now()
	s := &SpriteState{Name: "test"}

	changed := s.RecordSuccess("WORKING", now)
	if changed {
		t.Error("first poll (empty → WORKING) should not be a transition")
	}
}

func TestRecordFailure_BackoffProgression(t *testing.T) {
	now := time.Now()
	base := 15 * time.Second
	s := &SpriteState{Name: "test", Status: "WORKING"}

	// First failure: backoff = base (15s)
	s.RecordFailure(base, now)
	if s.ConsecFails != 1 {
		t.Errorf("ConsecFails = %d, want 1", s.ConsecFails)
	}
	expected := now.Add(15 * time.Second)
	if !s.BackoffUntil.Equal(expected) {
		t.Errorf("BackoffUntil = %v, want %v", s.BackoffUntil, expected)
	}

	// Second failure: backoff = 30s
	s.RecordFailure(base, now)
	if s.ConsecFails != 2 {
		t.Errorf("ConsecFails = %d, want 2", s.ConsecFails)
	}
	expected = now.Add(30 * time.Second)
	if !s.BackoffUntil.Equal(expected) {
		t.Errorf("BackoffUntil = %v, want %v", s.BackoffUntil, expected)
	}

	// Third failure: backoff = 60s, status → UNREACHABLE
	s.RecordFailure(base, now)
	if s.ConsecFails != 3 {
		t.Errorf("ConsecFails = %d, want 3", s.ConsecFails)
	}
	if s.Status != "UNREACHABLE" {
		t.Errorf("Status = %q, want UNREACHABLE after 3 failures", s.Status)
	}
}

func TestRecordFailure_MaxBackoff(t *testing.T) {
	now := time.Now()
	base := 15 * time.Second
	s := &SpriteState{Name: "test", Status: "WORKING"}

	// Simulate many failures to hit cap
	for i := 0; i < 20; i++ {
		s.RecordFailure(base, now)
	}

	maxExpected := now.Add(MaxBackoff)
	if s.BackoffUntil.After(maxExpected) {
		t.Errorf("BackoffUntil %v exceeds MaxBackoff %v", s.BackoffUntil, maxExpected)
	}
}

func TestRecordFailure_ThenSuccess_Resets(t *testing.T) {
	now := time.Now()
	base := 15 * time.Second
	s := &SpriteState{Name: "test", Status: "WORKING"}

	s.RecordFailure(base, now)
	s.RecordFailure(base, now)
	s.RecordFailure(base, now) // now UNREACHABLE

	s.RecordSuccess("WORKING", now)

	if s.ConsecFails != 0 {
		t.Errorf("ConsecFails = %d, want 0 after success", s.ConsecFails)
	}
	if s.Status != "WORKING" {
		t.Errorf("Status = %q, want WORKING after recovery", s.Status)
	}
}

func TestIsTransition(t *testing.T) {
	tests := []struct {
		name     string
		prev     string
		current  string
		want     bool
	}{
		{"empty previous", "", "WORKING", false},
		{"same status", "WORKING", "WORKING", false},
		{"different status", "WORKING", "WAITING", true},
		{"recovery", "UNREACHABLE", "WORKING", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpriteState{PreviousStatus: tt.prev, Status: tt.current}
			if got := s.IsTransition(); got != tt.want {
				t.Errorf("IsTransition() = %v, want %v", got, tt.want)
			}
		})
	}
}
