package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle-tui/internal/layout"
	"github.com/FacileStudio/nacelle-tui/internal/menu"
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
	body := strings.Join(append(above, m.prompt.View()), "\n")
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
	if len(m.parallelTasks) > 0 {
		above = append(above, m.parallelTasksView())
		above = append(above, "")
	}
	above = append(above, strings.Join(m.Queue.View(m.hist.Editing(m.Len()), m.width, m.theme.Question), "\n"))
	menuView := menu.View(&m.menu, max(m.width, 1), m.theme.Plain, m.theme.Menu, m.theme.Command)
	if menuView != "" {
		above = append(above, "", menuView)
	}
	return above
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
