package poller

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/JPM1118/slua/internal/sprites"
)

// safePattern matches only characters safe for shell grep -qE patterns.
// Allows alphanumeric, basic punctuation used in prompts, and regex operators.
var safePattern = regexp.MustCompile(`^[a-zA-Z0-9 /\\_\-\.\?\*\+\|\[\]\(\)\^]+$`)

// shellEscapePattern validates a pattern is safe for shell interpolation.
// Returns the pattern unchanged if safe, or an error if it contains
// characters that could break out of the grep expression.
func shellEscapePattern(pattern string) (string, error) {
	if pattern == "" {
		return "", fmt.Errorf("empty pattern")
	}
	if !safePattern.MatchString(pattern) {
		return "", fmt.Errorf("pattern contains unsafe characters: %q", pattern)
	}
	return pattern, nil
}

// BuildDetectionScript constructs the shell script used to detect
// Claude Code state on a Sprite. Prompt patterns are used to detect
// WAITING state. Patterns are validated against a safe character allowlist.
func BuildDetectionScript(patterns []string) string {
	escaped := make([]string, 0, len(patterns))
	for _, p := range patterns {
		safe, err := shellEscapePattern(p)
		if err != nil {
			continue // Skip unsafe patterns
		}
		escaped = append(escaped, safe)
	}

	// If all patterns were filtered out, use a pattern that never matches
	patternExpr := "^$NEVER_MATCH"
	if len(escaped) > 0 {
		patternExpr = strings.Join(escaped, "|")
	}

	return fmt.Sprintf(`if pgrep -a claude > /dev/null 2>&1; then
  RECENT=$(tmux capture-pane -p -l 5 2>/dev/null || echo "")
  if echo "$RECENT" | grep -qE "(%s)"; then
    echo "WAITING"
  else
    echo "WORKING"
  fi
else
  EXIT=$(tmux show-environment CLAUDE_EXIT 2>/dev/null | cut -d= -f2 || echo "")
  if [ "$EXIT" = "0" ] || [ -z "$EXIT" ]; then
    echo "FINISHED"
  else
    echo "ERROR:$EXIT"
  fi
fi`, patternExpr)
}

// Detect runs the status detection script on a single Sprite and
// returns a normalized status string.
func Detect(ctx context.Context, cli sprites.SpriteSource, name string, script string) (string, string, error) {
	output, err := cli.ExecStatus(ctx, name, script)
	if err != nil {
		return "", "", err
	}

	status, detail := ParseDetectionOutput(output)
	return status, detail, nil
}

// ParseDetectionOutput parses the raw output from the detection script
// into a status constant and optional detail string.
func ParseDetectionOutput(output string) (status string, detail string) {
	output = strings.TrimSpace(output)

	switch {
	case output == "WORKING":
		return sprites.StatusWorking, ""
	case output == "WAITING":
		return sprites.StatusWaiting, ""
	case output == "FINISHED":
		return sprites.StatusFinished, ""
	case strings.HasPrefix(output, "ERROR:"):
		code := strings.TrimPrefix(output, "ERROR:")
		return sprites.StatusError, code
	default:
		// Unparseable output → conservative SLEEPING default
		return sprites.StatusSleeping, ""
	}
}
