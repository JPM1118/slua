package sprites

import "os/exec"

// SpriteSource provides sprite data and console access.
// CLI implements this interface. Tests can provide mock implementations.
type SpriteSource interface {
	List() ([]Sprite, error)
	ConsoleCmd(name string) *exec.Cmd
}
