package notify

import (
	"fmt"
	"time"
)

// Notification represents a single state transition event.
type Notification struct {
	SpriteName string
	OldStatus  string
	NewStatus  string
	Timestamp  time.Time
}

// Bar manages a FIFO queue of notification entries.
type Bar struct {
	items    []Notification
	maxStore int
}

// NewBar creates a notification bar with the given buffer size.
func NewBar(maxStore int) *Bar {
	return &Bar{
		items:    make([]Notification, 0, maxStore),
		maxStore: maxStore,
	}
}

// Push adds a notification, trimming oldest if at capacity.
func (b *Bar) Push(n Notification) {
	b.items = append(b.items, n)
	if len(b.items) > b.maxStore {
		b.items = b.items[len(b.items)-b.maxStore:]
	}
}

// Visible returns the most recent notifications (max 2).
func (b *Bar) Visible() []Notification {
	if len(b.items) <= 2 {
		return b.items
	}
	return b.items[len(b.items)-2:]
}

// ClearForSprite removes all notifications for the given Sprite.
func (b *Bar) ClearForSprite(name string) {
	filtered := b.items[:0]
	for _, n := range b.items {
		if n.SpriteName != name {
			filtered = append(filtered, n)
		}
	}
	b.items = filtered
}

// Len returns the total number of buffered notifications.
func (b *Bar) Len() int {
	return len(b.items)
}

// Render formats the visible notifications for display within the given width.
func (b *Bar) Render(width int, now time.Time) string {
	visible := b.Visible()
	if len(visible) == 0 {
		return ""
	}

	parts := make([]string, 0, len(visible))
	for _, n := range visible {
		text := formatNotification(n, now)
		parts = append(parts, text)
	}

	// Join with separator, truncate to width
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " │ "
		}
		result += p
	}

	runes := []rune(result)
	if len(runes) > width {
		if width > 1 {
			result = string(runes[:width-1]) + "…"
		} else {
			result = string(runes[:width])
		}
	}

	return result
}

func formatNotification(n Notification, now time.Time) string {
	age := now.Sub(n.Timestamp).Truncate(time.Second)
	var ageStr string
	if age < time.Minute {
		ageStr = fmt.Sprintf("%ds ago", int(age.Seconds()))
	} else if age < time.Hour {
		ageStr = fmt.Sprintf("%dm ago", int(age.Minutes()))
	} else {
		ageStr = fmt.Sprintf("%dh ago", int(age.Hours()))
	}

	return fmt.Sprintf("● %s: %s → %s (%s)", n.SpriteName, n.OldStatus, n.NewStatus, ageStr)
}
