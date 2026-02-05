package tui

import (
	"fmt"
	"strings"

	"github.com/JPM1118/slua/internal/sprites"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	colName     = 24
	colStatus   = 14
	colUptime   = 11
	minWidth    = 80
	minHeight   = 24
	headerLines = 4 // header + subheader + column header + separator
	footerLines = 2 // status bar + notification bar
)

// Messages

type spritesLoadedMsg struct {
	sprites []sprites.Sprite
	err     error
}

type consoleFinishedMsg struct {
	err error
}

// Dashboard is the main Bubble Tea model.
type Dashboard struct {
	cli        sprites.SpriteSource
	sprites    []sprites.Sprite
	cursor     int
	width      int
	height     int
	err        error
	loading    bool
	lastErr    string // transient error shown in notification bar
}

// NewDashboard creates a new dashboard model.
func NewDashboard(cli sprites.SpriteSource) Dashboard {
	return Dashboard{
		cli:     cli,
		loading: true,
	}
}

// Err returns any fatal error that occurred.
func (d Dashboard) Err() error {
	return d.err
}

// Init loads the initial sprite list.
func (d Dashboard) Init() tea.Cmd {
	return d.loadSprites()
}

func (d Dashboard) loadSprites() tea.Cmd {
	return func() tea.Msg {
		spriteList, err := d.cli.List()
		return spritesLoadedMsg{sprites: spriteList, err: err}
	}
}

// Update handles messages.
func (d Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		return d.handleKey(msg)

	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
		return d, nil

	case spritesLoadedMsg:
		d.loading = false
		if msg.err != nil {
			if d.sprites == nil {
				// First load failed — show error
				d.lastErr = msg.err.Error()
			} else {
				// Refresh failed — keep stale data, show warning
				d.lastErr = fmt.Sprintf("Refresh failed: %s", msg.err.Error())
			}
		} else {
			d.sprites = msg.sprites
			d.lastErr = ""
		}
		// Clamp cursor
		if d.cursor >= len(d.sprites) {
			d.cursor = max(0, len(d.sprites)-1)
		}
		return d, nil

	case consoleFinishedMsg:
		if msg.err != nil {
			d.lastErr = fmt.Sprintf("Console error: %s", msg.err.Error())
		}
		// Refresh after returning from console
		return d, d.loadSprites()
	}

	return d, nil
}

func (d Dashboard) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return d, tea.Quit

	case "j", "down":
		if d.cursor < len(d.sprites)-1 {
			d.cursor++
		}
		return d, nil

	case "k", "up":
		if d.cursor > 0 {
			d.cursor--
		}
		return d, nil

	case "enter":
		if len(d.sprites) == 0 {
			return d, nil
		}
		s := d.sprites[d.cursor]
		c := d.cli.ConsoleCmd(s.Name)
		return d, tea.ExecProcess(c, func(err error) tea.Msg {
			return consoleFinishedMsg{err: err}
		})

	case "r":
		d.loading = true
		return d, d.loadSprites()

	case "G":
		if len(d.sprites) > 0 {
			d.cursor = len(d.sprites) - 1
		}
		return d, nil

	case "g":
		d.cursor = 0
		return d, nil
	}

	return d, nil
}

// View renders the dashboard.
func (d Dashboard) View() string {
	if d.width < minWidth || d.height < minHeight {
		return fmt.Sprintf("\n  Terminal too small (need %dx%d, got %dx%d)\n", minWidth, minHeight, d.width, d.height)
	}

	var b strings.Builder

	// Header
	b.WriteString(d.renderHeader())
	b.WriteString("\n")

	// Subheader
	b.WriteString(d.renderSubheader())
	b.WriteString("\n")

	// Column headers
	b.WriteString(d.renderColumnHeaders())
	b.WriteString("\n")

	// Separator
	b.WriteString(d.renderSeparator())
	b.WriteString("\n")

	// Sprite list
	listHeight := d.height - headerLines - footerLines
	b.WriteString(d.renderSpriteList(listHeight))

	// Notification bar
	b.WriteString(d.renderNotificationBar())
	b.WriteString("\n")

	// Status bar
	b.WriteString(d.renderStatusBar())

	return b.String()
}

func (d Dashboard) renderHeader() string {
	title := headerStyle.Render("Slua Sí")

	// Count attention-needing sprites
	attention := 0
	for _, s := range d.sprites {
		if s.Status == "WAITING" || s.Status == "ERROR" {
			attention++
		}
	}

	right := ""
	if attention > 0 {
		right = badgeStyle.Render(fmt.Sprintf("[%d need attention]", attention))
	}

	gap := d.width - lipgloss.Width(title) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	return title + strings.Repeat(" ", gap) + right
}

func (d Dashboard) renderSubheader() string {
	status := "Connected"
	if d.lastErr != "" {
		status = "Error"
	}
	if d.loading {
		status = "Loading..."
	}
	return subheaderStyle.Render(status)
}

func (d Dashboard) renderColumnHeaders() string {
	showActivity := d.width >= 100
	name := padRight("NAME", colName)
	st := padRight("STATUS", colStatus)
	up := padRight("UPTIME", colUptime)

	header := name + st + up
	if showActivity {
		header += "LAST ACTIVITY"
	}
	return columnHeaderStyle.Render(header)
}

func (d Dashboard) renderSeparator() string {
	showActivity := d.width >= 100
	name := padRight(strings.Repeat("─", colName-1), colName)
	st := padRight(strings.Repeat("─", colStatus-1), colStatus)
	up := padRight(strings.Repeat("─", colUptime-1), colUptime)

	sep := name + st + up
	if showActivity {
		sep += strings.Repeat("─", 16)
	}
	return subheaderStyle.Render(sep)
}

func (d Dashboard) renderSpriteList(height int) string {
	if d.loading && len(d.sprites) == 0 {
		return padLines("  Loading sprites...\n", height)
	}

	if len(d.sprites) == 0 {
		msg := "  No Sprites running.\n\n  Use 'sprite create <name>' to get started.\n"
		return padLines(msg, height)
	}

	showActivity := d.width >= 100

	// Calculate visible range (scroll if needed)
	start := 0
	if d.cursor >= height {
		start = d.cursor - height + 1
	}
	end := start + height
	if end > len(d.sprites) {
		end = len(d.sprites)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		s := d.sprites[i]

		// Cursor indicator
		prefix := "  "
		if i == d.cursor {
			prefix = cursorStyle.Render("▸ ")
		}

		name := truncate(s.Name, colName-2)
		name = padRight(name, colName-2) // -2 for prefix

		label := statusLabel(s.Status)
		styledStatus := statusStyle(s.Status).Render(padRight(label, colStatus))

		uptime := padRight(s.FormatUptime(), colUptime)

		line := prefix + name + styledStatus + uptime
		if showActivity {
			activity := activityText(s)
			line += lipgloss.NewStyle().Foreground(colorMuted).Render(activity)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Pad remaining lines
	rendered := end - start
	for i := rendered; i < height; i++ {
		b.WriteString("\n")
	}

	return b.String()
}

func (d Dashboard) renderNotificationBar() string {
	if d.lastErr != "" {
		return notificationBarStyle.Render("  " + truncate(d.lastErr, d.width-4))
	}
	return notificationBarStyle.Render("")
}

func (d Dashboard) renderStatusBar() string {
	return statusBarStyle.Render("  j/k:navigate  Enter:connect  r:refresh  q:quit")
}

// Helpers

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}

func padLines(content string, height int) string {
	lines := strings.Count(content, "\n")
	padding := height - lines
	if padding > 0 {
		content += strings.Repeat("\n", padding)
	}
	return content
}

func activityText(s sprites.Sprite) string {
	switch s.Status {
	case "WORKING":
		return "active"
	case "FINISHED":
		return "completed"
	case "WAITING":
		return "needs input"
	case "ERROR":
		return "failed"
	case "SLEEPING":
		return "idle"
	case "UNREACHABLE":
		return "connection lost"
	default:
		return ""
	}
}

