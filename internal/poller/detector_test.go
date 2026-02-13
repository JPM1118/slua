package poller

import (
	"testing"
)

func TestParseDetectionOutput(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantStatus string
		wantDetail string
	}{
		{"working", "WORKING", "WORKING", ""},
		{"waiting", "WAITING", "WAITING", ""},
		{"finished", "FINISHED", "FINISHED", ""},
		{"error with code", "ERROR:1", "ERROR", "1"},
		{"error with signal", "ERROR:137", "ERROR", "137"},
		{"empty output", "", "SLEEPING", ""},
		{"garbage output", "some random text", "SLEEPING", ""},
		{"working with whitespace", "  WORKING  \n", "WORKING", ""},
		{"waiting with newline", "WAITING\n", "WAITING", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, detail := ParseDetectionOutput(tt.output)
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
			if detail != tt.wantDetail {
				t.Errorf("detail = %q, want %q", detail, tt.wantDetail)
			}
		})
	}
}

func TestBuildDetectionScript(t *testing.T) {
	script := BuildDetectionScript([]string{"Y/n", "y/N", "Permission"})

	// Should contain the patterns in a grep expression
	if got := script; got == "" {
		t.Fatal("script should not be empty")
	}

	// Verify patterns are included
	for _, p := range []string{"Y/n", "y/N", "Permission"} {
		if !contains(script, p) {
			t.Errorf("script should contain pattern %q", p)
		}
	}

	// Verify basic structure
	for _, fragment := range []string{"pgrep -a claude", "tmux capture-pane", "WAITING", "WORKING", "FINISHED", "ERROR"} {
		if !contains(script, fragment) {
			t.Errorf("script should contain %q", fragment)
		}
	}
}

func TestBuildDetectionScript_EmptyPatterns(t *testing.T) {
	script := BuildDetectionScript([]string{})
	// Should still produce a valid script with empty pattern group
	if script == "" {
		t.Fatal("script should not be empty even with no patterns")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsCheck(s, substr))
}

func containsCheck(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
