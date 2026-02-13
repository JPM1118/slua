package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for slua.
type Config struct {
	Detection     DetectionConfig     `yaml:"detection"`
	Notifications NotificationConfig  `yaml:"notifications"`
}

// DetectionConfig controls how Sprites are polled for state.
type DetectionConfig struct {
	PollInterval   Duration `yaml:"poll_interval"`
	ExecTimeout    Duration `yaml:"exec_timeout"`
	PromptPatterns []string `yaml:"prompt_patterns"`
}

// NotificationConfig controls how the user is notified of state changes.
type NotificationConfig struct {
	TerminalBell bool     `yaml:"terminal_bell"`
	BellDebounce Duration `yaml:"bell_debounce"`
	BellOnStates []string `yaml:"bell_on_states"`
}

// Duration wraps time.Duration for YAML unmarshalling from strings like "15s".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// Defaults returns a Config with sensible default values.
func Defaults() Config {
	return Config{
		Detection: DetectionConfig{
			PollInterval: Duration{15 * time.Second},
			ExecTimeout:  Duration{5 * time.Second},
			PromptPatterns: []string{
				"Y/n",
				"y/N",
				`\? `,
				"> $",
				"Permission",
				"Allow",
				"Deny",
			},
		},
		Notifications: NotificationConfig{
			TerminalBell: true,
			BellDebounce: Duration{30 * time.Second},
			BellOnStates: []string{"WAITING", "ERROR"},
		},
	}
}

// Load reads the config file and merges with defaults.
// Missing file is not an error — defaults are used silently.
func Load() (Config, error) {
	return LoadFrom(configPath())
}

// LoadFrom reads config from a specific path.
func LoadFrom(path string) (Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Defaults(), fmt.Errorf("parse config %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return Defaults(), fmt.Errorf("config validation: %w", err)
	}

	// Filter out invalid regex patterns
	cfg.Detection.PromptPatterns = filterValidPatterns(cfg.Detection.PromptPatterns)

	return cfg, nil
}

func (c Config) validate() error {
	pi := c.Detection.PollInterval.Duration
	if pi < 5*time.Second || pi > 5*time.Minute {
		return fmt.Errorf("poll_interval must be between 5s and 5m, got %s", pi)
	}

	et := c.Detection.ExecTimeout.Duration
	if et < 2*time.Second || et > pi {
		return fmt.Errorf("exec_timeout must be between 2s and poll_interval (%s), got %s", pi, et)
	}

	return nil
}

func filterValidPatterns(patterns []string) []string {
	valid := make([]string, 0, len(patterns))
	for _, p := range patterns {
		if _, err := regexp.Compile(p); err == nil {
			valid = append(valid, p)
		}
	}
	return valid
}

func configPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "slua", "config.yml")
}
