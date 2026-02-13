package poller

import (
	"context"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/JPM1118/slua/internal/sprites"
)

// mockSource implements sprites.SpriteSource for poller testing.
type mockSource struct {
	mu         sync.Mutex
	sprites    []sprites.Sprite
	listErr    error
	execResult string
	execErr    error
	execCalls  int
}

func (m *mockSource) List(_ context.Context) ([]sprites.Sprite, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sprites, m.listErr
}

func (m *mockSource) ConsoleCmd(name string) *exec.Cmd {
	return exec.Command("echo", name)
}

func (m *mockSource) ExecStatus(_ context.Context, _ string, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execCalls++
	return m.execResult, m.execErr
}

func TestPoller_ReceivesUpdate(t *testing.T) {
	src := &mockSource{
		sprites: []sprites.Sprite{
			{Name: "web-app", Status: sprites.StatusWorking},
		},
		execResult: "WORKING",
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
	src := &mockSource{
		sprites: []sprites.Sprite{
			{Name: "web-app", Status: sprites.StatusWorking},
		},
		execResult: "WORKING",
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
	src.mu.Lock()
	src.execResult = "WAITING"
	src.mu.Unlock()

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
	src := &mockSource{
		sprites: []sprites.Sprite{
			{Name: "test", Status: sprites.StatusWorking},
		},
		execResult: "FINISHED",
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
	src := &mockSource{
		sprites: []sprites.Sprite{
			{Name: "sleeping", Status: sprites.StatusSleeping},
			{Name: "creating", Status: sprites.StatusCreating},
		},
		execResult: "WORKING",
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

	src.mu.Lock()
	calls := src.execCalls
	src.mu.Unlock()

	if calls != 0 {
		t.Errorf("ExecStatus called %d times, want 0 (no running sprites)", calls)
	}
}

func TestPoller_Stop(t *testing.T) {
	src := &mockSource{
		sprites: []sprites.Sprite{
			{Name: "test", Status: sprites.StatusWorking},
		},
		execResult: "WORKING",
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
