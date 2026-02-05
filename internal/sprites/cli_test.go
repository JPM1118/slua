package sprites

import (
	"testing"
	"time"
)

func TestParseSpritesJSON_Array(t *testing.T) {
	data := `[
		{"id": "sp1", "name": "my-app", "status": "running", "created_at": "2026-02-05T10:00:00Z", "region": "ord"},
		{"id": "sp2", "name": "api-dev", "status": "stopped", "created_at": "2026-02-05T08:00:00Z", "region": "sjc"}
	]`

	sprites, err := parseSpritesJSON([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprites) != 2 {
		t.Fatalf("expected 2 sprites, got %d", len(sprites))
	}

	if sprites[0].Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", sprites[0].Name)
	}
	if sprites[0].Status != StatusWorking {
		t.Errorf("expected status %q, got %q", StatusWorking, sprites[0].Status)
	}
	if sprites[0].Region != "ord" {
		t.Errorf("expected region 'ord', got %q", sprites[0].Region)
	}
	if sprites[0].CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}

	if sprites[1].Name != "api-dev" {
		t.Errorf("expected name 'api-dev', got %q", sprites[1].Name)
	}
	if sprites[1].Status != StatusSleeping {
		t.Errorf("expected status %q, got %q", StatusSleeping, sprites[1].Status)
	}
}

func TestParseSpritesJSON_WrappedObject(t *testing.T) {
	data := `{"data": [{"id": "sp1", "name": "test", "status": "running"}]}`

	sprites, err := parseSpritesJSON([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprites) != 1 {
		t.Fatalf("expected 1 sprite, got %d", len(sprites))
	}
	if sprites[0].Name != "test" {
		t.Errorf("expected name 'test', got %q", sprites[0].Name)
	}
}

func TestParseSpritesJSON_SpritesKey(t *testing.T) {
	data := `{"sprites": [{"id": "sp1", "name": "test", "status": "stopped"}]}`

	sprites, err := parseSpritesJSON([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprites) != 1 {
		t.Fatalf("expected 1 sprite, got %d", len(sprites))
	}
	if sprites[0].Status != StatusSleeping {
		t.Errorf("expected status %q, got %q", StatusSleeping, sprites[0].Status)
	}
}

func TestParseSpritesJSON_Empty(t *testing.T) {
	sprites, err := parseSpritesJSON([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sprites != nil {
		t.Errorf("expected nil for empty input, got %v", sprites)
	}
}

func TestParseSpritesJSON_EmptyArray(t *testing.T) {
	sprites, err := parseSpritesJSON([]byte("[]"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprites) != 0 {
		t.Errorf("expected 0 sprites, got %d", len(sprites))
	}
}

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"running", StatusWorking},
		{"started", StatusWorking},
		{"stopped", StatusSleeping},
		{"suspended", StatusSleeping},
		{"sleeping", StatusSleeping},
		{"destroyed", StatusDestroying},
		{"destroying", StatusDestroying},
		{"creating", StatusCreating},
		{"", StatusSleeping},
		{"unknown", "UNKNOWN"},
	}

	for _, tt := range tests {
		got := normalizeStatus(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeStatus(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSprite_FormatUptime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		createdAt time.Time
		expected  string
	}{
		{"zero time", time.Time{}, "—"},
		{"minutes only", now.Add(-30 * time.Minute), "30m"},
		{"hours and minutes", now.Add(-2*time.Hour - 15*time.Minute), "2h 15m"},
	}

	for _, tt := range tests {
		s := Sprite{CreatedAt: tt.createdAt}
		got := s.FormatUptime()
		if got != tt.expected {
			t.Errorf("%s: FormatUptime() = %q, want %q", tt.name, got, tt.expected)
		}
	}
}

func TestSprite_FormatUptime_HoursFormat(t *testing.T) {
	s := Sprite{CreatedAt: time.Now().Add(-5*time.Hour - 3*time.Minute)}
	got := s.FormatUptime()
	if got != "5h 03m" {
		t.Errorf("FormatUptime() = %q, want %q", got, "5h 03m")
	}
}
