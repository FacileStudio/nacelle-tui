package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle-tui/internal/layout"
	"github.com/FacileStudio/nacelle-tui/internal/theme"
)

type screen struct {
	width        int
	windowHeight int
	liveRows     int
	frameRows    int
}

func (m *Model) View() tea.View {
	view := m.assembleView()
	view.DisableBracketedPasteMode = true
	return view
}

func (m *Model) assembleView() tea.View {
	above := m.aboveContent()
	aboveHeight := lipgloss.Height(strings.Join(above, "\n"))
	parts := append(above, m.prompt.View())
	if below := m.belowContent(); below != "" {
		parts = append(parts, "", below)
	}
	body := strings.Join(parts, "\n")
	m.frameRows = lipgloss.Height(body)

	view := tea.NewView(body)
	if position := m.prompt.Cursor(); position != nil {
		position.Y += aboveHeight
		view.Cursor = position
	}
	return view
}

func (m *Model) aboveContent() []string {
	var above []string
	above = append(above, m.streaming()...)
	above = append(above, "")
	if tasksView := strings.Join(m.tasks.View(max(m.width, 1), m.theme.Muted), "\n"); tasksView != "" {
		above = append(above, tasksView)
		above = append(above, "")
	}
	above = append(above, m.status())
	above = append(above, strings.Join(m.Queue.View(m.hist.Editing(m.Len()), m.width, m.theme.Question), "\n"))
	return above
}

// belowContent is everything rendered beneath the prompt: the running parallel
// subagents and the slash-command suggestions. They sit here together so none
// fights for the same rows, and so the menu's own blank-line separator applies
// to the whole block rather than doubling between them.
func (m *Model) belowContent() string {
	var below []string
	if len(m.parallelTasks) > 0 {
		below = append(below, m.parallelTasksView())
	}
	if menu := m.viewMenu(); menu != "" {
		below = append(below, menu)
	}
	if len(below) == 0 {
		return ""
	}
	return strings.Join(below, "\n")
}

func (m *Model) resize(size tea.WindowSizeMsg) tea.Cmd {
	widthChanged := size.Width != m.width
	m.width, m.windowHeight = size.Width, size.Height

	m.prompt.SetWidth(size.Width)
	m.prompt.MaxHeight = layout.PromptCap(size.Height)
	m.prompt.SetHeight(m.prompt.Height())
	m.layout(size.Height)

	if widthChanged {
		m.restyle()
	}
	return nil
}

// retheme swaps the palette for a terminal background-colour report and
// rebuilds the renderer, so route's dispatcher stays one line per arm.
func (m *Model) retheme(message tea.BackgroundColorMsg) tea.Cmd {
	m.theme = theme.Themed(message.IsDark())
	m.restyle()
	return nil
}

func (m *Model) layout(height int) {
	taken := 3 + m.prompt.Height() + m.menu.Height() + m.Height(m.hist.Editing(m.Len())) + m.tasks.Rows() + m.parallelTasksRows()
	m.liveRows = layout.LiveRows(height, taken)
}

func (m *Model) printed(text string) tea.Cmd {
	budget := layout.Budget(m.windowHeight, m.frameRows)
	batches := layout.Batches(text, budget, m.width)
	cmds := make([]tea.Cmd, 0, len(batches))
	for _, batch := range batches {
		cmds = append(cmds, tea.Println(batch))
	}
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Sequence(cmds...)
}
