package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/JPM1118/slua/internal/sprites"
	tea "github.com/charmbracelet/bubbletea"
)

// mockSource implements sprites.SpriteSource for testing.
type mockSource struct {
	sprites []sprites.Sprite
	err     error
}

func (m *mockSource) List() ([]sprites.Sprite, error) {
	return m.sprites, m.err
}

func (m *mockSource) ConsoleCmd(name string) *exec.Cmd {
	return exec.Command("echo", name)
}

// testDashboard creates a Dashboard with mock data already loaded.
func testDashboard(src sprites.SpriteSource, width, height int) Dashboard {
	d := NewDashboard(src)
	d.width = width
	d.height = height

	// Simulate Init() completing
	cmd := d.Init()
	if cmd != nil {
		msg := cmd()
		updated, _ := d.Update(msg)
		d = updated.(Dashboard)
	}
	return d
}

func keyMsg(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	if s == "ctrl+c" {
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestView_ShowsAllSprites(t *testing.T) {
	now := time.Now()
	src := &mockSource{
		sprites: []sprites.Sprite{
			{Name: "web-app", Status: "WORKING", CreatedAt: now.Add(-2 * time.Hour)},
			{Name: "api-dev", Status: "SLEEPING", CreatedAt: now.Add(-45 * time.Minute)},
			{Name: "docs", Status: "FINISHED", CreatedAt: now.Add(-30 * time.Minute)},
		},
	}
	d := testDashboard(src, 100, 30)
	view := d.View()

	for _, name := range []string{"web-app", "api-dev", "docs"} {
		if !strings.Contains(view, name) {
			t.Errorf("View() missing sprite name %q", name)
		}
	}

	for _, status := range []string{"WORKING", "SLEEPING", "FINISHED"} {
		if !strings.Contains(view, status) {
			t.Errorf("View() missing status %q", status)
		}
	}

	if !strings.Contains(view, "2h") {
		t.Errorf("View() missing uptime for web-app")
	}
}

func TestUpdate_JKNavigation(t *testing.T) {
	src := &mockSource{
		sprites: []sprites.Sprite{
			{Name: "first"},
			{Name: "second"},
			{Name: "third"},
		},
	}
	d := testDashboard(src, 100, 30)

	if d.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", d.cursor)
	}

	// j -> 1
	updated, _ := d.Update(keyMsg("j"))
	d = updated.(Dashboard)
	if d.cursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", d.cursor)
	}

	// j -> 2
	updated, _ = d.Update(keyMsg("j"))
	d = updated.(Dashboard)
	if d.cursor != 2 {
		t.Errorf("after j j: cursor = %d, want 2", d.cursor)
	}

	// j -> stays at 2 (clamped)
	updated, _ = d.Update(keyMsg("j"))
	d = updated.(Dashboard)
	if d.cursor != 2 {
		t.Errorf("after j j j: cursor = %d, want 2 (clamped)", d.cursor)
	}

	// k -> 1
	updated, _ = d.Update(keyMsg("k"))
	d = updated.(Dashboard)
	if d.cursor != 1 {
		t.Errorf("after k: cursor = %d, want 1", d.cursor)
	}

	// k -> 0
	updated, _ = d.Update(keyMsg("k"))
	d = updated.(Dashboard)
	if d.cursor != 0 {
		t.Errorf("after k k: cursor = %d, want 0", d.cursor)
	}

	// k -> stays at 0 (clamped)
	updated, _ = d.Update(keyMsg("k"))
	d = updated.(Dashboard)
	if d.cursor != 0 {
		t.Errorf("after k k k: cursor = %d, want 0 (clamped)", d.cursor)
	}
}

func TestUpdate_GAndgNavigation(t *testing.T) {
	src := &mockSource{
		sprites: []sprites.Sprite{
			{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"},
		},
	}
	d := testDashboard(src, 100, 30)

	// G -> jump to bottom
	updated, _ := d.Update(keyMsg("G"))
	d = updated.(Dashboard)
	if d.cursor != 3 {
		t.Errorf("after G: cursor = %d, want 3", d.cursor)
	}

	// g -> jump to top
	updated, _ = d.Update(keyMsg("g"))
	d = updated.(Dashboard)
	if d.cursor != 0 {
		t.Errorf("after g: cursor = %d, want 0", d.cursor)
	}
}

func TestUpdate_QuitOnQ(t *testing.T) {
	src := &mockSource{sprites: []sprites.Sprite{{Name: "test"}}}
	d := testDashboard(src, 100, 30)

	_, cmd := d.Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("q should return a command")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("q command returned %T, want tea.QuitMsg", msg)
	}
}

func TestUpdate_EnterReturnsExec(t *testing.T) {
	src := &mockSource{
		sprites: []sprites.Sprite{{Name: "my-sprite"}},
	}
	d := testDashboard(src, 100, 30)

	_, cmd := d.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("Enter should return a command")
	}
}

func TestUpdate_EnterOnEmptyList(t *testing.T) {
	src := &mockSource{sprites: []sprites.Sprite{}}
	d := testDashboard(src, 100, 30)

	_, cmd := d.Update(keyMsg("enter"))
	if cmd != nil {
		t.Errorf("Enter on empty list should return nil cmd, got non-nil")
	}
}

func TestView_TerminalTooSmall(t *testing.T) {
	src := &mockSource{sprites: []sprites.Sprite{{Name: "test"}}}
	d := testDashboard(src, 40, 10)
	view := d.View()

	if !strings.Contains(view, "Terminal too small") {
		t.Errorf("View() should show too-small message, got: %s", view)
	}
}

func TestView_EmptySprites(t *testing.T) {
	src := &mockSource{sprites: []sprites.Sprite{}}
	d := testDashboard(src, 100, 30)
	view := d.View()

	if !strings.Contains(view, "No Sprites running") {
		t.Errorf("View() should show empty message")
	}
}

func TestView_LoadingState(t *testing.T) {
	src := &mockSource{sprites: []sprites.Sprite{}}
	d := NewDashboard(src)
	d.width = 100
	d.height = 30
	// Don't run Init() — loading is still true
	view := d.View()

	if !strings.Contains(view, "Loading") {
		t.Errorf("View() should show loading state, got: %s", view)
	}
}

func TestView_ErrorInNotificationBar(t *testing.T) {
	src := &mockSource{err: fmt.Errorf("auth expired")}
	d := testDashboard(src, 100, 30)
	view := d.View()

	if !strings.Contains(view, "auth expired") {
		t.Errorf("View() should show error in notification bar")
	}
}

func TestView_AttentionBadge(t *testing.T) {
	src := &mockSource{
		sprites: []sprites.Sprite{
			{Name: "ok", Status: "WORKING"},
			{Name: "waiting", Status: "WAITING"},
			{Name: "broken", Status: "ERROR"},
		},
	}
	d := testDashboard(src, 100, 30)
	view := d.View()

	if !strings.Contains(view, "2 need attention") {
		t.Errorf("View() should show attention badge")
	}
}

func TestView_ResponsiveActivityColumn(t *testing.T) {
	src := &mockSource{
		sprites: []sprites.Sprite{
			{Name: "worker", Status: "WORKING"},
		},
	}

	// Wide: activity column visible
	d := testDashboard(src, 110, 30)
	wide := d.View()
	if !strings.Contains(wide, "LAST ACTIVITY") {
		t.Errorf("wide View() should show LAST ACTIVITY column")
	}
	if !strings.Contains(wide, "active") {
		t.Errorf("wide View() should show activity text 'active'")
	}

	// Narrow: activity column hidden
	d = testDashboard(src, 90, 30)
	narrow := d.View()
	if strings.Contains(narrow, "LAST ACTIVITY") {
		t.Errorf("narrow View() should hide LAST ACTIVITY column")
	}
}

func TestView_CursorIndicator(t *testing.T) {
	src := &mockSource{
		sprites: []sprites.Sprite{
			{Name: "first-sprite"},
			{Name: "second-sprite"},
		},
	}
	d := testDashboard(src, 100, 30)
	view := d.View()

	if !strings.Contains(view, "▸") {
		t.Errorf("View() should show cursor indicator")
	}
}

func TestUpdate_WindowResize(t *testing.T) {
	src := &mockSource{sprites: []sprites.Sprite{{Name: "test"}}}
	d := testDashboard(src, 100, 30)

	updated, _ := d.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	d = updated.(Dashboard)

	if d.width != 120 || d.height != 40 {
		t.Errorf("after resize: %dx%d, want 120x40", d.width, d.height)
	}
}
