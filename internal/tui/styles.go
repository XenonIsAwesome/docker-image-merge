package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	ColorA = lipgloss.Color("#2ECC71")
	ColorB = lipgloss.Color("#3498DB")
	ColorConflict = lipgloss.Color("#E74C3C")
	ColorResolved = lipgloss.Color("#F39C12")
	ColorUnresolved = lipgloss.Color("#95A5A6")
	ColorSkipped = lipgloss.Color("#E67E22")
	ColorBg = lipgloss.Color("#2C3E50")
	ColorFg = lipgloss.Color("#ECF0F1")
	ColorMuted = lipgloss.Color("#7F8C8D")
	ColorHighlight = lipgloss.Color("#F1C40F")
	ColorBorder = lipgloss.Color("#34495E")

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorHighlight).
			Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorFg).
			Background(ColorBg).
			Padding(0, 1)

	KeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorHighlight)

	ConflictItemStyle = lipgloss.NewStyle().
				Padding(0, 1)

	ConflictActiveStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#34495E")).
				Bold(true)

	ResolvedAStyle = lipgloss.NewStyle().
			Foreground(ColorA).
			Padding(0, 1)

	ResolvedBStyle = lipgloss.NewStyle().
			Foreground(ColorB).
			Padding(0, 1)

	UnresolvedStyle = lipgloss.NewStyle().
			Foreground(ColorUnresolved).
			Padding(0, 1)

	SkippedStyle = lipgloss.NewStyle().
			Foreground(ColorSkipped).
			Padding(0, 1)

	PaneHeaderA = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorA).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorA)

	PaneHeaderB = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorB).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorB)

	DiffLineAdded = lipgloss.NewStyle().
			Foreground(ColorA)

	DiffLineRemoved = lipgloss.NewStyle().
			Foreground(ColorConflict)

	DiffLineContext = lipgloss.NewStyle().
			Foreground(ColorMuted)

	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(1, 0)

	SummaryStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorFg).
			Padding(1, 0)
)

func RenderConflictIndicator(resolved, total int) string {
	return fmt.Sprintf("[%d/%d resolved]", resolved, total)
}

func RenderStatusLine(current, total int, strategy string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Conflict %d/%d", current+1, total))
	b.WriteString("  |  ")
	b.WriteString(fmt.Sprintf("Strategy: %s", strategy))
	return StatusBarStyle.Render(b.String())
}

func RenderHelpBar() string {
	keys := []string{
		"[a] Take A",
		"[b] Take B",
		"[s] Skip",
		"[e] Editor",
		"[A] All A",
		"[B] All B",
		"[n/p] Navigate",
		"[q] Quit",
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = KeyStyle.Render(k[:3]) + k[3:]
	}
	return HelpStyle.Render(strings.Join(parts, "  "))
}
