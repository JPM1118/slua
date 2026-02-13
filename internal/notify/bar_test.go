package notify

import (
	"strings"
	"testing"
	"time"
)

func TestBar_PushAndVisible(t *testing.T) {
	b := NewBar(20)
	now := time.Now()

	b.Push(Notification{SpriteName: "a", OldStatus: "SLEEPING", NewStatus: "WORKING", Timestamp: now})
	b.Push(Notification{SpriteName: "b", OldStatus: "WORKING", NewStatus: "WAITING", Timestamp: now})
	b.Push(Notification{SpriteName: "c", OldStatus: "WORKING", NewStatus: "FINISHED", Timestamp: now})

	visible := b.Visible()
	if len(visible) != 2 {
		t.Fatalf("Visible() = %d items, want 2", len(visible))
	}
	if visible[0].SpriteName != "b" {
		t.Errorf("visible[0].SpriteName = %q, want b", visible[0].SpriteName)
	}
	if visible[1].SpriteName != "c" {
		t.Errorf("visible[1].SpriteName = %q, want c", visible[1].SpriteName)
	}
}

func TestBar_VisibleWithOneItem(t *testing.T) {
	b := NewBar(20)
	b.Push(Notification{SpriteName: "only"})

	visible := b.Visible()
	if len(visible) != 1 {
		t.Fatalf("Visible() = %d items, want 1", len(visible))
	}
}

func TestBar_VisibleEmpty(t *testing.T) {
	b := NewBar(20)
	if len(b.Visible()) != 0 {
		t.Error("empty bar should have no visible items")
	}
}

func TestBar_MaxBuffer(t *testing.T) {
	b := NewBar(3)
	now := time.Now()

	for i := 0; i < 10; i++ {
		b.Push(Notification{SpriteName: string(rune('a' + i)), Timestamp: now})
	}

	if b.Len() != 3 {
		t.Errorf("Len() = %d, want 3 (max buffer)", b.Len())
	}

	visible := b.Visible()
	// Should be the last 2 of the last 3
	if visible[0].SpriteName != "i" {
		t.Errorf("visible[0] = %q, want i", visible[0].SpriteName)
	}
	if visible[1].SpriteName != "j" {
		t.Errorf("visible[1] = %q, want j", visible[1].SpriteName)
	}
}

func TestBar_ClearForSprite(t *testing.T) {
	b := NewBar(20)
	now := time.Now()

	b.Push(Notification{SpriteName: "keep", Timestamp: now})
	b.Push(Notification{SpriteName: "remove", Timestamp: now})
	b.Push(Notification{SpriteName: "keep", Timestamp: now})
	b.Push(Notification{SpriteName: "remove", Timestamp: now})

	b.ClearForSprite("remove")

	if b.Len() != 2 {
		t.Errorf("after clear: Len() = %d, want 2", b.Len())
	}
	for _, n := range b.Visible() {
		if n.SpriteName == "remove" {
			t.Error("cleared sprite should not appear in visible items")
		}
	}
}

func TestBar_Render(t *testing.T) {
	b := NewBar(20)
	now := time.Now()

	b.Push(Notification{
		SpriteName: "web-app",
		OldStatus:  "WORKING",
		NewStatus:  "WAITING",
		Timestamp:  now.Add(-2 * time.Minute),
	})

	result := b.Render(80, now)
	if !strings.Contains(result, "web-app") {
		t.Errorf("render should contain sprite name, got: %q", result)
	}
	if !strings.Contains(result, "WORKING") || !strings.Contains(result, "WAITING") {
		t.Errorf("render should contain old/new status, got: %q", result)
	}
	if !strings.Contains(result, "2m ago") {
		t.Errorf("render should contain relative time, got: %q", result)
	}
}

func TestBar_RenderEmpty(t *testing.T) {
	b := NewBar(20)
	if b.Render(80, time.Now()) != "" {
		t.Error("empty bar should render empty string")
	}
}

func TestBar_RenderTruncation(t *testing.T) {
	b := NewBar(20)
	now := time.Now()

	b.Push(Notification{SpriteName: "very-long-name", OldStatus: "WORKING", NewStatus: "WAITING", Timestamp: now})
	b.Push(Notification{SpriteName: "another-long-name", OldStatus: "WORKING", NewStatus: "ERROR", Timestamp: now})

	result := b.Render(30, now)
	runes := []rune(result)
	if len(runes) > 30 {
		t.Errorf("render should be truncated to 30 runes, got %d: %q", len(runes), result)
	}
}
