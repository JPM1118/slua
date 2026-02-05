package tui

import "github.com/charmbracelet/lipgloss"

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
)

// statusStyle returns the appropriate style for a Sprite status.
func statusStyle(status string) lipgloss.Style {
	switch status {
	case "WORKING":
		return lipgloss.NewStyle().Foreground(colorWorking)
	case "FINISHED":
		return lipgloss.NewStyle().Foreground(colorFinished)
	case "WAITING":
		return lipgloss.NewStyle().Foreground(colorWaiting).Bold(true)
	case "ERROR":
		return lipgloss.NewStyle().Foreground(colorError).Bold(true)
	case "SLEEPING":
		return lipgloss.NewStyle().Foreground(colorSleeping)
	case "UNREACHABLE":
		return lipgloss.NewStyle().Foreground(colorUnreachable)
	default:
		return lipgloss.NewStyle().Foreground(colorMuted)
	}
}

// statusLabel returns the display text for a status, including indicators.
func statusLabel(status string) string {
	switch status {
	case "ERROR":
		return "ERROR !"
	case "UNREACHABLE":
		return "UNREACHABLE ?"
	case "FINISHED":
		return "FINISHED"
	default:
		return status
	}
}
