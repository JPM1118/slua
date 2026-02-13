package sprites

import (
	"context"
	"os/exec"
)

// SpriteSource provides sprite data and console access.
// CLI implements this interface. Tests can provide mock implementations.
type SpriteSource interface {
	List(ctx context.Context) ([]Sprite, error)
	ConsoleCmd(name string) *exec.Cmd
	ExecStatus(ctx context.Context, name string, script string) (string, error)
}
