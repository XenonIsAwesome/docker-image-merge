// Package tui implements the interactive BubbleTea-based conflict resolver.
//
// It presents a full-screen TUI with a side-by-side diff view for each
// conflicting file, letting the user choose which version to keep or skip.
// Keyboard shortcuts allow bulk-resolution of all remaining conflicts.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette for the TUI.
var (
	ColorA          = lipgloss.Color("#2ECC71") // green — image A
	ColorB          = lipgloss.Color("#3498DB") // blue — image B
	ColorConflict   = lipgloss.Color("#E74C3C") // red — conflicts
	ColorResolved   = lipgloss.Color("#F39C12") // orange — resolved
	ColorUnresolved = lipgloss.Color("#95A5A6") // grey — unresolved
	ColorSkipped    = lipgloss.Color("#E67E22") // dark orange — skipped
	ColorBg         = lipgloss.Color("#2C3E50") // dark background
	ColorFg         = lipgloss.Color("#ECF0F1") // light foreground
	ColorMuted      = lipgloss.Color("#7F8C8D") // muted text
	ColorHighlight  = lipgloss.Color("#F1C40F") // yellow — active key
	ColorBorder     = lipgloss.Color("#34495E") // border color
)

// Pre-built lipgloss styles for various UI elements.
var (
	// TitleStyle renders the conflict header (e.g. "Conflict 3/17: /etc/foo").
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorHighlight).
			Padding(0, 1)

	// StatusBarStyle renders the bottom status bar background.
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorFg).
			Background(ColorBg).
			Padding(0, 1)

	// KeyStyle highlights a key binding label in the help bar.
	KeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorHighlight)

	// ConflictItemStyle styles a single item in the conflict list sidebar.
	ConflictItemStyle = lipgloss.NewStyle().
				Padding(0, 1)

	// ConflictActiveStyle highlights the currently focused conflict.
	ConflictActiveStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#34495E")).
				Bold(true)

	// ResolvedAStyle renders text in image A's color (green).
	ResolvedAStyle = lipgloss.NewStyle().
			Foreground(ColorA).
			Padding(0, 1)

	// ResolvedBStyle renders text in image B's color (blue).
	ResolvedBStyle = lipgloss.NewStyle().
			Foreground(ColorB).
			Padding(0, 1)

	// UnresolvedStyle renders unresolved items in grey.
	UnresolvedStyle = lipgloss.NewStyle().
			Foreground(ColorUnresolved).
			Padding(0, 1)

	// SkippedStyle renders skipped items in dark orange.
	SkippedStyle = lipgloss.NewStyle().
			Foreground(ColorSkipped).
			Padding(0, 1)

	// PaneHeaderA styles the "Image A" header in the diff pane.
	PaneHeaderA = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorA).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorA)

	// PaneHeaderB styles the "Image B" header in the diff pane.
	PaneHeaderB = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorB).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorB)

	// DiffLineAdded highlights added lines in green.
	DiffLineAdded = lipgloss.NewStyle().
			Foreground(ColorA)

	// DiffLineRemoved highlights removed lines in red.
	DiffLineRemoved = lipgloss.NewStyle().
			Foreground(ColorConflict)

	// DiffLineContext renders context lines in muted grey.
	DiffLineContext = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// HelpStyle renders the bottom help bar.
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(1, 0)

	// SummaryStyle renders the final merge summary header.
	SummaryStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorFg).
			Padding(1, 0)
)

// RenderConflictIndicator returns a string like "[3/17 resolved]" for the status bar.
func RenderConflictIndicator(resolved, total int) string {
	return fmt.Sprintf("[%d/%d resolved]", resolved, total)
}

// RenderStatusLine returns a formatted status line showing the current
// conflict number, total, and active strategy.
func RenderStatusLine(current, total int, strategy string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Conflict %d/%d", current+1, total))
	b.WriteString("  |  ")
	b.WriteString(fmt.Sprintf("Strategy: %s", strategy))
	return StatusBarStyle.Render(b.String())
}

// RenderHelpBar returns the bottom help bar with all key binding hints.
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
		// Highlight the key portion (first 3 chars like "[a]") in yellow.
		parts[i] = KeyStyle.Render(k[:3]) + k[3:]
	}
	return HelpStyle.Render(strings.Join(parts, "  "))
}
