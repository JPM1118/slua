package cmd

import (
	"fmt"

	"github.com/JPM1118/slua/internal/sprites"
	"github.com/JPM1118/slua/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Launch the interactive TUI dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDashboard()
	},
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}

func runDashboard() error {
	cli := &sprites.CLI{Org: org}

	model := tui.NewDashboard(cli)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("dashboard: %w", err)
	}

	if m, ok := finalModel.(tui.Dashboard); ok && m.Err() != nil {
		return m.Err()
	}
	return nil
}
