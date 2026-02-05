package tui

import (
	"github.com/JPM1118/slua/internal/sprites"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	colorWorking     = lipgloss.Color("3")  // yellow
	colorFinished    = lipgloss.Color("2")  // green
	colorWaiting     = lipgloss.Color("1")  // red
	colorError       = lipgloss.Color("1")  // red
	colorSleeping    = lipgloss.Color("8")  // dim gray
	colorUnreachable = lipgloss.Color("8")  // dim gray
	colorHeader      = lipgloss.Color("12") // bright blue
	colorMuted       = lipgloss.Color("8")  // dim
	colorCursor      = lipgloss.Color("6")  // cyan

	// Styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorHeader)

	subheaderStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	cursorStyle = lipgloss.NewStyle().
			Foreground(colorCursor).
			Bold(true)

	columnHeaderStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Underline(true)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	notificationBarStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Italic(true)

	badgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Pre-allocated status styles
	statusStyleWorking     = lipgloss.NewStyle().Foreground(colorWorking)
	statusStyleFinished    = lipgloss.NewStyle().Foreground(colorFinished)
	statusStyleWaiting     = lipgloss.NewStyle().Foreground(colorWaiting).Bold(true)
	statusStyleError       = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	statusStyleSleeping    = lipgloss.NewStyle().Foreground(colorSleeping)
	statusStyleUnreachable = lipgloss.NewStyle().Foreground(colorUnreachable)
	statusStyleDefault     = lipgloss.NewStyle().Foreground(colorMuted)
)

// statusStyle returns the appropriate style for a Sprite status.
func statusStyle(status string) lipgloss.Style {
	switch status {
	case sprites.StatusWorking:
		return statusStyleWorking
	case sprites.StatusFinished:
		return statusStyleFinished
	case sprites.StatusWaiting:
		return statusStyleWaiting
	case sprites.StatusError:
		return statusStyleError
	case sprites.StatusSleeping:
		return statusStyleSleeping
	case sprites.StatusUnreachable:
		return statusStyleUnreachable
	default:
		return statusStyleDefault
	}
}

// statusLabel returns the display text for a status, including indicators.
func statusLabel(status string) string {
	switch status {
	case sprites.StatusError:
		return "ERROR !"
	case sprites.StatusUnreachable:
		return "UNREACHABLE ?"
	default:
		return status
	}
}
