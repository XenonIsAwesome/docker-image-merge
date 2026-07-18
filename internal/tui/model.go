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

type Model struct {
	conflicts   []*merge.Conflict
	current     int
	diffView    string
	quitting    bool
	confirmed   bool
	width       int
	height      int
	allA        bool
	allB        bool
}

type resolveMsg struct{ index int }
type doneMsg struct{}

func NewModel(conflicts []*merge.Conflict) Model {
	m := Model{
		conflicts: conflicts,
		current:   0,
		width:     120,
		height:    40,
	}
	m.updateDiff()
	return m
}

func (m *Model) updateDiff() {
	if m.current >= len(m.conflicts) {
		m.diffView = ""
		return
	}

	c := m.conflicts[m.current]
	if c.InfoA != nil && c.InfoB != nil && c.InfoA.AbsPath != "" && c.InfoB.AbsPath != "" {
		m.diffView = merge.GenerateDiff(c.InfoA.AbsPath, c.InfoB.AbsPath)
	} else {
		m.diffView = fmt.Sprintf("File: %s\nKind: %s\n", c.Path, c.Kind)
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

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
		for i := m.current; i < len(m.conflicts); i++ {
			if m.conflicts[i].Resolution == merge.ResolutionNone {
				m.conflicts[i].Resolution = merge.ResolutionTakeA
			}
		}
		m.confirmed = true
		return m, tea.Quit

	case "B":
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
			m.confirmed = true
			return m, tea.Quit
		}
		m.moveToNext()
		return m, nil
	}

	return m, nil
}

func (m *Model) moveToNext() {
	for i := m.current + 1; i < len(m.conflicts); i++ {
		if m.conflicts[i].Resolution == merge.ResolutionNone {
			m.current = i
			m.updateDiff()
			return
		}
	}

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

func (m *Model) moveToPrev() {
	for i := m.current - 1; i >= 0; i-- {
		if m.conflicts[i].Resolution == merge.ResolutionNone || m.conflicts[i].Resolution == merge.ResolutionSkip {
			m.current = i
			m.updateDiff()
			return
		}
	}
}

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

		tmpFile, err := os.CreateTemp("", "merge-*.txt")
		if err != nil {
			return doneMsg{}
		}
		defer os.Remove(tmpFile.Name())

		content := fmt.Sprintf("<<<<<<< Image A\n%s\n=======\n%s\n>>>>>>> Image B\n",
		 readFileContent(c.InfoA.AbsPath),
		 readFileContent(c.InfoB.AbsPath))
		tmpFile.WriteString(content)
		tmpFile.Close()

		cmd := exec.Command(editor, tmpFile.Name())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()

		edited, _ := os.ReadFile(tmpFile.Name())
		if strings.Contains(string(edited), "<<<<<<<") {
			c.Resolution = merge.ResolutionTakeA
		} else {
			c.Resolution = merge.ResolutionTakeB
		}

		return resolveMsg{index: m.current}
	}
}

func readFileContent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(error reading file: %v)", err)
	}
	return string(data)
}

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

func (m Model) renderConflictView() string {
	var b strings.Builder

	title := TitleStyle.Render(fmt.Sprintf("Conflict %d/%d: %s", m.current+1, len(m.conflicts), m.conflicts[m.current].Path))
	b.WriteString(title)
	b.WriteString("\n\n")

	paneWidth := (m.width - 4) / 2
	if paneWidth < 20 {
		paneWidth = 20
	}

	leftPane := m.renderPane("Image A", m.conflicts[m.current].InfoA, paneWidth, ColorA)
	rightPane := m.renderPane("Image B", m.conflicts[m.current].InfoB, paneWidth, ColorB)

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPane, "  ", rightPane))
	b.WriteString("\n\n")

	statusLine := RenderStatusLine(m.current, len(m.conflicts), string(m.conflicts[m.current].Kind))
	b.WriteString(statusLine)
	b.WriteString("\n")

	b.WriteString(RenderHelpBar())

	return b.String()
}

func (m Model) renderPane(label string, info *merge.FileInfo, width int, color lipgloss.Color) string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(color).
		Width(width).
		Render(label)

	var content strings.Builder
	if info == nil {
		content.WriteString("(not present)")
	} else if info.IsDir {
		content.WriteString(fmt.Sprintf("[directory]\n%s", info.RelPath))
	} else if info.IsSymlink {
		content.WriteString(fmt.Sprintf("[symlink] -> %s", info.SymlinkTarget))
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
			content.WriteString(fmt.Sprintf("Size: %d bytes\nMode: %o", info.Size, info.Mode))
		}
	}

	pane := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Width(width).
		Height(m.height - 8).
		Padding(0, 1).
		Render(header + "\n" + content.String())

	return pane
}

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

	b.WriteString(TitleStyle.Render("Merge Summary"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Total conflicts:  %d\n", len(m.conflicts)))
	b.WriteString(ResolvedAStyle.Render(fmt.Sprintf("Take A:           %d", takeA)))
	b.WriteString("\n")
	b.WriteString(ResolvedBStyle.Render(fmt.Sprintf("Take B:           %d", takeB)))
	b.WriteString("\n")
	b.WriteString(SkippedStyle.Render(fmt.Sprintf("Skipped:          %d", skipped)))
	b.WriteString("\n")

	if unresolved > 0 {
		b.WriteString(fmt.Sprintf("Unresolved:       %d\n", unresolved))
	}

	b.WriteString("\n")
	if m.confirmed {
		b.WriteString(SummaryStyle.Render("Resolutions applied. Building merged image..."))
	} else {
		b.WriteString(SummaryStyle.Render("Cancelled. No changes applied."))
	}

	return b.String()
}

func Run(conflicts []*merge.Conflict) (bool, error) {
	needsResolution := false
	for _, c := range conflicts {
		if c.Kind.NeedsResolution() {
			needsResolution = true
			break
		}
	}

	if !needsResolution {
		for _, c := range conflicts {
			if c.Kind == merge.OnlyB {
				c.Resolution = merge.ResolutionTakeB
			} else if c.Kind == merge.OnlyA {
				c.Resolution = merge.ResolutionTakeA
			}
		}
		return true, nil
	}

	p := tea.NewProgram(
		NewModel(conflicts),
		tea.WithAltScreen(),
	)

	finalModel, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("running TUI: %w", err)
	}

	m := finalModel.(Model)
	return m.confirmed, nil
}
