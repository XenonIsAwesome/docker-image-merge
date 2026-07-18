package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/XenonIsAwesome/docker-image-merge/internal/merge"
)

// Model is the BubbleTea model for the interactive conflict resolver.
//
// It maintains a list of conflicts, the index of the currently displayed
// conflict, and the generated diff text. Keyboard events drive state
// transitions; the final model's resolutions are read back after Quit.
type Model struct {
	// conflicts is the filtered list of conflicts needing resolution.
	conflicts []*merge.Conflict

	// current is the index into conflicts of the conflict being displayed.
	current int

	// diffView is the pre-formatted side-by-side diff text for the current conflict.
	diffView string

	// imageA is the name/tag of image A (the base image).
	imageA string

	// imageB is the name/tag of image B (the incoming image).
	imageB string

	// quitting is set when the user presses q or Ctrl+C to abort.
	quitting bool

	// confirmed is set when all conflicts are resolved and the user exits.
	confirmed bool

	// width is the terminal width in columns.
	width int

	// height is the terminal height in rows.
	height int

	// totalDiffs is the total number of diffs (including auto-resolved).
	totalDiffs int

	// autoResolvedA is the count of non-conflicting diffs auto-resolved as TakeA.
	autoResolvedA int

	// autoResolvedB is the count of non-conflicting diffs auto-resolved as TakeB.
	autoResolvedB int
}

// NewModel creates a Model pre-loaded with the given conflicts and computes
// the initial diff view.
func NewModel(conflicts []*merge.Conflict, imageA, imageB string, totalDiffs, autoA, autoB int) Model {
	m := Model{
		conflicts:     conflicts,
		current:       0,
		imageA:        imageA,
		imageB:        imageB,
		width:         120,
		height:        40,
		totalDiffs:    totalDiffs,
		autoResolvedA: autoA,
		autoResolvedB: autoB,
	}
	m.updateDiff()
	return m
}

// updateDiff regenerates the diffView string for the current conflict.
func (m *Model) updateDiff() {
	if m.current >= len(m.conflicts) {
		m.diffView = ""
		return
	}

	c := m.conflicts[m.current]
	if c.InfoA != nil && c.InfoB != nil && c.InfoA.AbsPath != "" && c.InfoB.AbsPath != "" {
		m.diffView = merge.GenerateDiff(c.InfoA.AbsPath, c.InfoB.AbsPath, m.imageA, m.imageB)
	} else {
		m.diffView = fmt.Sprintf("File: %s\nKind: %s\n", c.Path, c.Kind)
	}
}

// Init implements tea.Model. No startup command is needed.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. It dispatches to handleKey for keyboard events
// and handles window resize events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey processes a single key press and returns the updated model.
//
// Supported keys:
//
//	a/b    — resolve current conflict with A or B
//	s      — skip current conflict (keeps A)
//	e      — open $EDITOR for manual merge
//	A/B    — resolve ALL remaining conflicts with A or B
//	n/→    — move to next unresolved conflict
//	p/←    — move to previous conflict
//	q/Ctrl+C — abort without applying
//	Enter/Space — advance to next (or finish if all resolved)
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "a":
		if m.current < len(m.conflicts) {
			m.conflicts[m.current].Resolution = merge.ResolutionTakeA
			m.moveToNext()
		}
		return m, nil

	case "b":
		if m.current < len(m.conflicts) {
			m.conflicts[m.current].Resolution = merge.ResolutionTakeB
			m.moveToNext()
		}
		return m, nil

	case "s":
		if m.current < len(m.conflicts) {
			m.conflicts[m.current].Resolution = merge.ResolutionSkip
			m.moveToNext()
		}
		return m, nil

	case "e":
		if m.current < len(m.conflicts) {
			return m, m.openEditor()
		}
		return m, nil

	case "A":
		// Bulk-resolve all remaining conflicts with A's version.
		for i := m.current; i < len(m.conflicts); i++ {
			if m.conflicts[i].Resolution == merge.ResolutionNone {
				m.conflicts[i].Resolution = merge.ResolutionTakeA
			}
		}
		m.confirmed = true
		return m, tea.Quit

	case "B":
		// Bulk-resolve all remaining conflicts with B's version.
		for i := m.current; i < len(m.conflicts); i++ {
			if m.conflicts[i].Resolution == merge.ResolutionNone {
				m.conflicts[i].Resolution = merge.ResolutionTakeB
			}
		}
		m.confirmed = true
		return m, tea.Quit

	case "n", "right":
		m.moveToNext()
		return m, nil

	case "p", "left":
		m.moveToPrev()
		return m, nil

	case "enter", " ":
		if m.current >= len(m.conflicts)-1 {
			// Last conflict — finish.
			m.confirmed = true
			return m, tea.Quit
		}
		m.moveToNext()
		return m, nil
	}

	return m, nil
}

// moveToNext advances to the next unresolved conflict, or marks as confirmed
// if all conflicts are resolved.
func (m *Model) moveToNext() {
	for i := m.current + 1; i < len(m.conflicts); i++ {
		if m.conflicts[i].Resolution == merge.ResolutionNone {
			m.current = i
			m.updateDiff()
			return
		}
	}

	// No more unresolved conflicts — check if everything is done.
	allResolved := true
	for _, c := range m.conflicts {
		if c.Resolution == merge.ResolutionNone {
			allResolved = false
			break
		}
	}
	if allResolved {
		m.confirmed = true
	}
}

// moveToPrev goes back to the previous conflict (resolved or not).
func (m *Model) moveToPrev() {
	for i := m.current - 1; i >= 0; i-- {
		if m.conflicts[i].Resolution == merge.ResolutionNone || m.conflicts[i].Resolution == merge.ResolutionSkip {
			m.current = i
			m.updateDiff()
			return
		}
	}
}

// openEditor opens $EDITOR with a 3-way merge template. It returns a tea.Cmd
// that runs synchronously and resolves the conflict based on whether conflict
// markers remain in the edited file.
func (m Model) openEditor() tea.Cmd {
	return func() tea.Msg {
		if m.current >= len(m.conflicts) {
			return doneMsg{}
		}

		c := m.conflicts[m.current]
		if c.InfoA == nil || c.InfoB == nil {
			return doneMsg{}
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		// Create a temp file with a 3-way merge template.
		tmpFile, err := os.CreateTemp("", "merge-*.txt")
		if err != nil {
			return doneMsg{}
		}
		defer os.Remove(tmpFile.Name()) //nolint:errcheck

		content := fmt.Sprintf("<<<<<<< Image A\n%s\n=======\n%s\n>>>>>>> Image B\n",
			readFileContent(c.InfoA.AbsPath),
			readFileContent(c.InfoB.AbsPath))
		_, _ = tmpFile.WriteString(content)
		_ = tmpFile.Close()

		// Launch the editor and wait for the user to finish.
		cmd := exec.Command(editor, tmpFile.Name())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()

		// If conflict markers remain, the user kept A; otherwise assume B.
		edited, _ := os.ReadFile(tmpFile.Name())
		if strings.Contains(string(edited), "<<<<<<<") {
			c.Resolution = merge.ResolutionTakeA
		} else {
			c.Resolution = merge.ResolutionTakeB
		}

		return resolveMsg{index: m.current}
	}
}

// readFileContent reads a file and returns its content as a string, or an
// error message if the read fails.
func readFileContent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(error reading file: %v)", err)
	}
	return string(data)
}

// View implements tea.Model. It renders the current conflict with a
// side-by-side diff view, status bar, and help bar.
func (m Model) View() string {
	if m.quitting {
		return "\nAborted.\n"
	}

	if len(m.conflicts) == 0 {
		return "\nNo conflicts found.\n"
	}

	if m.confirmed || m.current >= len(m.conflicts) {
		return m.renderSummary()
	}

	return m.renderConflictView()
}

// renderConflictView renders the full-screen conflict resolution UI.
func (m Model) renderConflictView() string {
	var b strings.Builder

	// Title bar showing the current conflict path.
	title := TitleStyle.Render(fmt.Sprintf("Conflict %d/%d: %s",
		m.current+1, len(m.conflicts), m.conflicts[m.current].Path))
	b.WriteString(title)
	b.WriteString("\n\n")

	// Two side-by-side panes showing A and B versions.
	paneWidth := (m.width - 4) / 2
	if paneWidth < 20 {
		paneWidth = 20
	}

	leftPane := m.renderPane(m.imageA, m.conflicts[m.current].InfoA, paneWidth, ColorA)
	rightPane := m.renderPane(m.imageB, m.conflicts[m.current].InfoB, paneWidth, ColorB)

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPane, "  ", rightPane))
	b.WriteString("\n\n")

	// Status line and help bar.
	statusLine := RenderStatusLine(m.current, len(m.conflicts),
		m.conflicts[m.current].Kind.String())
	b.WriteString(statusLine)
	b.WriteString("\n")
	b.WriteString(RenderHelpBar())

	return b.String()
}

// renderPane renders a single diff pane (either Image A or Image B) with a
// bordered box and the diff content.
func (m Model) renderPane(label string, info *merge.FileInfo, width int, color lipgloss.Color) string {
	// Pane header.
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(color).
		Width(width).
		Render(label)

	// Pane content — either the diff or file metadata.
	var content strings.Builder
	if info == nil {
		content.WriteString("(not present)")
	} else if info.IsDir {
		fmt.Fprintf(&content, "[directory]\n%s", info.RelPath)
	} else if info.IsSymlink {
		fmt.Fprintf(&content, "[symlink] -> %s", info.SymlinkTarget)
	} else {
		if m.diffView != "" && m.current < len(m.conflicts) {
			lines := strings.Split(m.diffView, "\n")
			visibleLines := m.height - 10
			if visibleLines < 1 {
				visibleLines = 20
			}
			start := 0
			end := len(lines)
			if end > visibleLines {
				end = visibleLines
			}
			for _, line := range lines[start:end] {
				if strings.HasPrefix(line, "--- Image A ---") || strings.HasPrefix(line, "+++ ") {
					content.WriteString(DiffLineRemoved.Render(line))
				} else if strings.HasPrefix(line, "--- Image B ---") || strings.HasPrefix(line, "--- ") {
					content.WriteString(DiffLineAdded.Render(line))
				} else if strings.HasPrefix(line, "+") {
					content.WriteString(DiffLineAdded.Render(line))
				} else if strings.HasPrefix(line, "-") {
					content.WriteString(DiffLineRemoved.Render(line))
				} else {
					content.WriteString(line)
				}
				content.WriteString("\n")
			}
		} else {
			fmt.Fprintf(&content, "Size: %d bytes\nMode: %o", info.Size, info.Mode)
		}
	}

	// Wrap in a bordered box.
	pane := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Width(width).
		Height(m.height - 8).
		Padding(0, 1).
		Render(header + "\n" + content.String())

	return pane
}

// renderSummary renders the final summary screen showing resolution statistics.
func (m Model) renderSummary() string {
	var b strings.Builder

	takeA, takeB, skipped, unresolved := 0, 0, 0, 0
	for _, c := range m.conflicts {
		switch c.Resolution {
		case merge.ResolutionTakeA:
			takeA++
		case merge.ResolutionTakeB:
			takeB++
		case merge.ResolutionSkip:
			skipped++
		default:
			unresolved++
		}
	}

	totalTakeA := takeA + m.autoResolvedA
	totalTakeB := takeB + m.autoResolvedB

	b.WriteString(TitleStyle.Render("Merge Summary"))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "Total diffs:      %d\n", m.totalDiffs)
	b.WriteString(ResolvedAStyle.Render(fmt.Sprintf("Take A:           %d", totalTakeA)))
	b.WriteString("\n")
	b.WriteString(ResolvedBStyle.Render(fmt.Sprintf("Take B:           %d", totalTakeB)))
	b.WriteString("\n")
	b.WriteString(SkippedStyle.Render(fmt.Sprintf("Skipped:          %d", skipped)))
	b.WriteString("\n")

	if m.autoResolvedA+m.autoResolvedB > 0 {
		fmt.Fprintf(&b, "Auto-resolved:    %d\n", m.autoResolvedA+m.autoResolvedB)
	}
	if unresolved > 0 {
		fmt.Fprintf(&b, "Unresolved:       %d\n", unresolved)
	}

	b.WriteString("\n")
	if m.confirmed {
		b.WriteString(SummaryStyle.Render("Resolutions applied. Building merged image..."))
	} else {
		b.WriteString(SummaryStyle.Render("Cancelled. No changes applied."))
	}

	return b.String()
}

// resolveMsg is an internal BubbleTea message sent after the editor resolves a conflict.
type resolveMsg struct{ index int }

// doneMsg is an internal BubbleTea message that signals completion.
type doneMsg struct{}

// Run launches the interactive TUI with the given conflicts and blocks until
// the user finishes. It returns (true, nil) if resolutions were confirmed,
// or (false, nil) if the user aborted.
//
// allDiffs is the full list of diffs (including auto-resolved OnlyA/OnlyB).
// conflicts is the filtered list of conflicts needing resolution.
func Run(conflicts []*merge.Conflict, allDiffs []*merge.Conflict, imageA, imageB string) (bool, error) {
	// Check if any conflicts actually need interactive resolution.
	needsResolution := false
	for _, c := range conflicts {
		if c.Kind.NeedsResolution() {
			needsResolution = true
			break
		}
	}

	// If nothing needs resolution, auto-resolve everything.
	if !needsResolution {
		for _, c := range allDiffs {
			switch c.Kind {
			case merge.OnlyB:
				c.Resolution = merge.ResolutionTakeB
			case merge.OnlyA:
				c.Resolution = merge.ResolutionTakeA
			}
		}
		return true, nil
	}

	// Count auto-resolved diffs for the summary.
	autoA, autoB := 0, 0
	for _, c := range allDiffs {
		if !c.Kind.NeedsResolution() {
			switch c.Resolution {
			case merge.ResolutionTakeA:
				autoA++
			case merge.ResolutionTakeB:
				autoB++
			}
		}
	}

	// Launch the full-screen BubbleTea TUI.
	p := tea.NewProgram(
		NewModel(conflicts, imageA, imageB, len(allDiffs), autoA, autoB),
		tea.WithAltScreen(),
	)

	finalModel, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("running TUI: %w", err)
	}

	m := finalModel.(Model)
	return m.confirmed, nil
}
