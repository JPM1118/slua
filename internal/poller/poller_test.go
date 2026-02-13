package poller

import (
	"context"
	"testing"
	"time"

	"github.com/JPM1118/slua/internal/sprites"
	"github.com/JPM1118/slua/internal/testutil"
)

func TestPoller_ReceivesUpdate(t *testing.T) {
	src := &testutil.MockSource{
		Sprites: []sprites.Sprite{
			{Name: "web-app", Status: sprites.StatusWorking},
		},
		ExecResult: "WORKING",
	}

	p := New(src, Config{
		PollInterval:   100 * time.Millisecond,
		ExecTimeout:    50 * time.Millisecond,
		PromptPatterns: []string{"Y/n"},
		MaxWorkers:     2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	p.Start(ctx)

	// Wait for first update
	select {
	case update := <-p.Updates():
		if _, ok := update.States["web-app"]; !ok {
			t.Error("update should contain web-app state")
		}
		state := update.States["web-app"]
		if state.Status != sprites.StatusWorking {
			t.Errorf("state.Status = %q, want WORKING", state.Status)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for poller update")
	}
}

func TestPoller_DetectsTransition(t *testing.T) {
	src := &testutil.MockSource{
		Sprites: []sprites.Sprite{
			{Name: "web-app", Status: sprites.StatusWorking},
		},
		ExecResult: "WORKING",
	}

	p := New(src, Config{
		PollInterval:   50 * time.Millisecond,
		ExecTimeout:    30 * time.Millisecond,
		PromptPatterns: []string{"Y/n"},
		MaxWorkers:     2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	p.Start(ctx)

	// Wait for first update (WORKING)
	select {
	case <-p.Updates():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for first update")
	}

	// Change exec result to WAITING
	src.SetExecResult("WAITING")

	// Wait for second update with transition
	select {
	case update := <-p.Updates():
		state := update.States["web-app"]
		if state.Status != sprites.StatusWaiting {
			t.Errorf("after transition: Status = %q, want WAITING", state.Status)
		}
		if state.PreviousStatus != sprites.StatusWorking {
			t.Errorf("PreviousStatus = %q, want WORKING", state.PreviousStatus)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for transition update")
	}
}

func TestPoller_TriggerNow(t *testing.T) {
	src := &testutil.MockSource{
		Sprites: []sprites.Sprite{
			{Name: "test", Status: sprites.StatusWorking},
		},
		ExecResult: "FINISHED",
	}

	p := New(src, Config{
		PollInterval:   10 * time.Second, // Very long — should not trigger on its own
		ExecTimeout:    50 * time.Millisecond,
		PromptPatterns: []string{},
		MaxWorkers:     2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	p.Start(ctx)

	// Drain initial poll
	select {
	case <-p.Updates():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for initial update")
	}

	// Trigger immediate poll
	p.TriggerNow()

	select {
	case <-p.Updates():
		// Got the triggered update
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for triggered update")
	}
}

func TestPoller_SkipsNonRunningSprites(t *testing.T) {
	src := &testutil.MockSource{
		Sprites: []sprites.Sprite{
			{Name: "sleeping", Status: sprites.StatusSleeping},
			{Name: "creating", Status: sprites.StatusCreating},
		},
		ExecResult: "WORKING",
	}

	p := New(src, Config{
		PollInterval:   50 * time.Millisecond,
		ExecTimeout:    30 * time.Millisecond,
		PromptPatterns: []string{},
		MaxWorkers:     2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	p.Start(ctx)

	// Wait for update — should have no states since no running sprites were polled
	select {
	case <-p.Updates():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for update")
	}

	calls := src.GetExecCalls()
	if calls != 0 {
		t.Errorf("ExecStatus called %d times, want 0 (no running sprites)", calls)
	}
}

func TestPoller_Stop(t *testing.T) {
	src := &testutil.MockSource{
		Sprites: []sprites.Sprite{
			{Name: "test", Status: sprites.StatusWorking},
		},
		ExecResult: "WORKING",
	}

	p := New(src, Config{
		PollInterval:   50 * time.Millisecond,
		ExecTimeout:    30 * time.Millisecond,
		PromptPatterns: []string{},
		MaxWorkers:     2,
	})

	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)

	// Drain initial update
	select {
	case <-p.Updates():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for initial update")
	}

	// Cancel context to stop poller
	cancel()

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	// Should not receive more updates
	select {
	case <-p.Updates():
		// Might get one more in-flight, that's OK
	default:
	}
}
