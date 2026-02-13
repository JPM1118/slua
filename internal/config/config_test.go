package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.Detection.PollInterval.Duration != 15*time.Second {
		t.Errorf("default poll_interval = %s, want 15s", cfg.Detection.PollInterval)
	}
	if cfg.Detection.ExecTimeout.Duration != 5*time.Second {
		t.Errorf("default exec_timeout = %s, want 5s", cfg.Detection.ExecTimeout)
	}
	if len(cfg.Detection.PromptPatterns) == 0 {
		t.Error("default prompt_patterns should not be empty")
	}
	if !cfg.Notifications.TerminalBell {
		t.Error("default terminal_bell should be true")
	}
	if cfg.Notifications.BellDebounce.Duration != 30*time.Second {
		t.Errorf("default bell_debounce = %s, want 30s", cfg.Notifications.BellDebounce)
	}
}

func TestLoadFrom_MissingFile(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/config.yml")
	if err != nil {
		t.Fatalf("missing file should not error, got: %v", err)
	}
	if cfg.Detection.PollInterval.Duration != 15*time.Second {
		t.Errorf("missing file should use defaults, got poll_interval = %s", cfg.Detection.PollInterval)
	}
}

func TestLoadFrom_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	data := `
detection:
  poll_interval: "30s"
  exec_timeout: "3s"
  prompt_patterns:
    - "custom pattern"
notifications:
  terminal_bell: false
  bell_debounce: "60s"
  bell_on_states:
    - "WAITING"
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("valid file should not error, got: %v", err)
	}
	if cfg.Detection.PollInterval.Duration != 30*time.Second {
		t.Errorf("poll_interval = %s, want 30s", cfg.Detection.PollInterval)
	}
	if cfg.Detection.ExecTimeout.Duration != 3*time.Second {
		t.Errorf("exec_timeout = %s, want 3s", cfg.Detection.ExecTimeout)
	}
	if len(cfg.Detection.PromptPatterns) != 1 || cfg.Detection.PromptPatterns[0] != "custom pattern" {
		t.Errorf("prompt_patterns = %v, want [custom pattern]", cfg.Detection.PromptPatterns)
	}
	if cfg.Notifications.TerminalBell {
		t.Error("terminal_bell should be false")
	}
	if cfg.Notifications.BellDebounce.Duration != 60*time.Second {
		t.Errorf("bell_debounce = %s, want 60s", cfg.Notifications.BellDebounce)
	}
}

func TestLoadFrom_PartialFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	data := `
detection:
  poll_interval: "20s"
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("partial file should not error, got: %v", err)
	}
	if cfg.Detection.PollInterval.Duration != 20*time.Second {
		t.Errorf("poll_interval = %s, want 20s", cfg.Detection.PollInterval)
	}
	// Partial file: exec_timeout should keep default
	if cfg.Detection.ExecTimeout.Duration != 5*time.Second {
		t.Errorf("exec_timeout should be default 5s, got %s", cfg.Detection.ExecTimeout)
	}
}

func TestLoadFrom_InvalidPollInterval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	data := `
detection:
  poll_interval: "1s"
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("poll_interval of 1s should fail validation")
	}
}

func TestLoadFrom_ExecTimeoutExceedsPollInterval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	data := `
detection:
  poll_interval: "10s"
  exec_timeout: "15s"
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("exec_timeout > poll_interval should fail validation")
	}
}

func TestLoadFrom_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte("{{not yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("malformed YAML should return error")
	}
}

func TestLoadFrom_InvalidRegexFiltered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	data := `
detection:
  prompt_patterns:
    - "valid"
    - "[invalid"
    - "also valid"
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("invalid regex should be filtered, not error: %v", err)
	}
	if len(cfg.Detection.PromptPatterns) != 2 {
		t.Errorf("should have 2 valid patterns, got %d: %v", len(cfg.Detection.PromptPatterns), cfg.Detection.PromptPatterns)
	}
}

func TestConfigPath_XDGOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	path := configPath()
	want := "/custom/config/slua/config.yml"
	if path != want {
		t.Errorf("configPath() = %q, want %q", path, want)
	}
}
