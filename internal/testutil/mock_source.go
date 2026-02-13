package testutil

import (
	"context"
	"os/exec"
	"sync"

	"github.com/JPM1118/slua/internal/sprites"
)

// MockSource implements sprites.SpriteSource for testing.
type MockSource struct {
	mu         sync.Mutex
	Sprites    []sprites.Sprite
	ListErr    error
	ExecResult string
	ExecErr    error
	ExecCalls  int
}

func (m *MockSource) List(_ context.Context) ([]sprites.Sprite, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Sprites, m.ListErr
}

func (m *MockSource) ConsoleCmd(name string) *exec.Cmd {
	return exec.Command("echo", name)
}

func (m *MockSource) ExecStatus(_ context.Context, _ string, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ExecCalls++
	return m.ExecResult, m.ExecErr
}

// SetExecResult updates the exec result in a thread-safe manner.
func (m *MockSource) SetExecResult(result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ExecResult = result
}

// GetExecCalls returns the number of ExecStatus calls in a thread-safe manner.
func (m *MockSource) GetExecCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ExecCalls
}
