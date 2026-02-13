package notify

import (
	"testing"
	"time"
)

func TestBell_RingOnTriggerState(t *testing.T) {
	b := NewBell(30*time.Second, []string{"WAITING", "ERROR"})
	now := time.Now()

	// WAITING should trigger
	if !b.Ring("WAITING", now) {
		t.Error("WAITING should trigger bell")
	}
}

func TestBell_NoRingOnNonTriggerState(t *testing.T) {
	b := NewBell(30*time.Second, []string{"WAITING", "ERROR"})
	now := time.Now()

	if b.Ring("WORKING", now) {
		t.Error("WORKING should not trigger bell")
	}
	if b.Ring("FINISHED", now) {
		t.Error("FINISHED should not trigger bell")
	}
}

func TestBell_Debounce(t *testing.T) {
	b := NewBell(30*time.Second, []string{"WAITING"})
	now := time.Now()

	// First ring should succeed
	if !b.Ring("WAITING", now) {
		t.Error("first ring should succeed")
	}

	// Second ring within debounce should fail
	if b.Ring("WAITING", now.Add(10*time.Second)) {
		t.Error("ring within debounce window should be suppressed")
	}

	// Ring after debounce should succeed
	if !b.Ring("WAITING", now.Add(31*time.Second)) {
		t.Error("ring after debounce window should succeed")
	}
}

func TestBell_Suspended(t *testing.T) {
	b := NewBell(30*time.Second, []string{"WAITING"})
	now := time.Now()

	b.Suspend()
	if b.Ring("WAITING", now) {
		t.Error("bell should not ring while suspended")
	}
	if !b.IsSuspended() {
		t.Error("IsSuspended should be true")
	}

	b.Resume()
	if b.IsSuspended() {
		t.Error("IsSuspended should be false after resume")
	}
}
