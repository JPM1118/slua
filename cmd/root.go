package cmd

import (
	"fmt"
	"os"

	"github.com/JPM1118/slua/internal/sprites"
	"github.com/spf13/cobra"
)

var org string

var rootCmd = &cobra.Command{
	Use:   "slua",
	Short: "Slua Sí — TUI orchestrator for Fly.io Sprite sessions",
	Long: `Slua Sí (Irish: "the fairy host") — a control tower for managing
multiple Fly.io Sprite instances running Claude Code.

Run without arguments to launch the dashboard.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return sprites.CheckSpriteCLI()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDashboard()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&org, "org", "o", "", "Fly.io organization to use")
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}
