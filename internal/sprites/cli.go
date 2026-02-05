package sprites

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Status constants used across the codebase.
const (
	StatusWorking     = "WORKING"
	StatusSleeping    = "SLEEPING"
	StatusFinished    = "FINISHED"
	StatusWaiting     = "WAITING"
	StatusError       = "ERROR"
	StatusUnreachable = "UNREACHABLE"
	StatusDestroying  = "DESTROYING"
	StatusCreating    = "CREATING"
)

// Sprite represents a remote Fly.io Sprite instance.
type Sprite struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Region    string    `json:"region"`
}

// Uptime returns the duration since the Sprite was created.
func (s Sprite) Uptime() time.Duration {
	if s.CreatedAt.IsZero() {
		return 0
	}
	return time.Since(s.CreatedAt).Truncate(time.Second)
}

// FormatUptime returns a human-readable uptime string like "2h 15m".
func (s Sprite) FormatUptime() string {
	d := s.Uptime()
	if d == 0 {
		return "—"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %02dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// CLI wraps the sprite command-line tool.
type CLI struct {
	// Org specifies the organization to use. Empty for default.
	Org string
}

var _ SpriteSource = (*CLI)(nil)

// ListTimeout is the default timeout for CLI.List operations.
const ListTimeout = 10 * time.Second

// spriteCmd builds a sprite command with org flag if set.
func (c *CLI) spriteCmd(ctx context.Context, args ...string) *exec.Cmd {
	if c.Org != "" {
		args = append([]string{"-o", c.Org}, args...)
	}
	return exec.CommandContext(ctx, "sprite", args...)
}

// List returns all Sprites in the configured organization.
// It uses `sprite api /sprites` to get JSON output.
func (c *CLI) List(ctx context.Context) ([]Sprite, error) {
	ctx, cancel := context.WithTimeout(ctx, ListTimeout)
	defer cancel()

	cmd := c.spriteCmd(ctx, "api", "/sprites")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("sprite api /sprites: %s", errMsg)
	}

	return parseSpritesJSON(stdout.Bytes())
}

// apiSprite matches the JSON structure returned by the Sprites API.
type apiSprite struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	Region    string `json:"region"`
}

// parseSpritesJSON parses the API response into Sprite structs.
// The API may return a JSON array or an object with a "sprites" key.
func parseSpritesJSON(data []byte) ([]Sprite, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}

	var apiSprites []apiSprite

	// Try array first
	if data[0] == '[' {
		if err := json.Unmarshal(data, &apiSprites); err != nil {
			return nil, fmt.Errorf("parse sprites JSON array: %w", err)
		}
	} else {
		// Try object with data/sprites key
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal(data, &wrapper); err != nil {
			return nil, fmt.Errorf("parse sprites JSON: %w", err)
		}
		// Try common keys
		for _, key := range []string{"data", "sprites"} {
			if raw, ok := wrapper[key]; ok {
				if err := json.Unmarshal(raw, &apiSprites); err == nil {
					break
				}
			}
		}
		if apiSprites == nil {
			return nil, fmt.Errorf("unexpected API response format")
		}
	}

	sprites := make([]Sprite, len(apiSprites))
	for i, as := range apiSprites {
		var createdAt time.Time
		if as.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, as.CreatedAt); err == nil {
				createdAt = t
			}
		}
		sprites[i] = Sprite{
			ID:        as.ID,
			Name:      as.Name,
			Status:    normalizeStatus(as.Status),
			CreatedAt: createdAt,
			Region:    as.Region,
		}
	}
	return sprites, nil
}

// normalizeStatus maps API status strings to display-friendly states.
func normalizeStatus(s string) string {
	switch strings.ToLower(s) {
	case "running", "started":
		return StatusWorking
	case "stopped", "suspended", "sleeping":
		return StatusSleeping
	case "destroyed", "destroying":
		return StatusDestroying
	case "creating":
		return StatusCreating
	default:
		if s == "" {
			return StatusSleeping
		}
		return strings.ToUpper(s)
	}
}

// ConsoleCmd returns an *exec.Cmd for `sprite console -s <name>`.
// The caller is responsible for setting Stdin/Stdout/Stderr and running it.
func (c *CLI) ConsoleCmd(name string) *exec.Cmd {
	return c.spriteCmd(context.Background(), "console", "-s", name)
}

// CheckSpriteCLI verifies that the sprite CLI is installed and accessible.
func CheckSpriteCLI() error {
	_, err := exec.LookPath("sprite")
	if err != nil {
		return fmt.Errorf("sprite CLI not found in PATH. Install it from https://sprites.dev")
	}
	return nil
}
