package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/JPM1118/slua/internal/config"
	"github.com/JPM1118/slua/internal/notify"
	"github.com/JPM1118/slua/internal/poller"
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
	cfg, err := config.Load()
	if err != nil {
		log.Printf("config warning: %v (using defaults)", err)
		cfg = config.Defaults()
	}

	cli := &sprites.CLI{Org: org}

	p := poller.New(cli, poller.Config{
		PollInterval:   cfg.Detection.PollInterval.Duration,
		ExecTimeout:    cfg.Detection.ExecTimeout.Duration,
		PromptPatterns: cfg.Detection.PromptPatterns,
		MaxWorkers:     10,
	})

	bell := notify.NewBell(
		cfg.Notifications.BellDebounce.Duration,
		cfg.Notifications.BellOnStates,
	)
	bar := notify.NewBar(20)

	model := tui.NewDashboard(cli,
		tui.WithPoller(p),
		tui.WithBell(bell),
		tui.WithNotifyBar(bar),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	program := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := program.Run()
	cancel() // Stop poller
	if err != nil {
		return fmt.Errorf("dashboard: %w", err)
	}

	if m, ok := finalModel.(tui.Dashboard); ok && m.Err() != nil {
		return m.Err()
	}
	return nil
}
