package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle-tui/internal/layout"
	"github.com/FacileStudio/nacelle-tui/internal/menu"
)

const promptRows = 10

type screen struct {
	width        int
	windowHeight int
	liveRows     int
	frameRows    int
}

func newPrompt() textarea.Model {
	prompt := textarea.New()
	prompt.Placeholder = "Ask something. Esc stops a run, ctrl+c stops or quits, ctrl+\\ forces it."
	prompt.SetPromptFunc(2, continuation)
	prompt.ShowLineNumbers = false
	prompt.DynamicHeight = true
	prompt.MinHeight = 1
	prompt.MaxHeight = promptRows
	prompt.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "shift+enter", "ctrl+j"))
	prompt.SetVirtualCursor(false)
	prompt.Focus()
	return prompt
}

func continuation(info textarea.PromptInfo) string {
	if info.LineNumber == 0 {
		return "> "
	}
	return "  "
}

func (m *Model) ask() tea.Cmd {
	question := strings.TrimSpace(m.prompt.Value())
	if question == "" {
		return nil
	}
	m.prompt.Reset()

	held := m.hist.Requeue(m.Items(), question)
	if !held && m.run.busy {
		m.Add(question)
		held = true
	}
	m.hist.Remember(question, m.Items())
	m.layout(m.windowHeight)
	if held {
		return nil
	}
	return m.dispatch(question)
}

func (m *Model) dispatch(line string) tea.Cmd {
	m.say(fromReader, line)

	started := tea.Cmd(nil)
	if cmd, ok := m.parseCommand(line); ok {
		started = cmd(m)
	} else {
		started = m.send(line)
	}
	return tea.Sequence(m.prints(), started)
}

func (m *Model) View() tea.View {
	above := append(m.streaming(), "")
	above = append(above, m.tasks.View(max(m.width, 1), m.theme.Muted)...)
	if m.tasks != nil {
		above = append(above, "")
	}
	above = append(above, m.status())
	above = append(above, m.Queue.View(m.hist.Editing(m.Len()), m.width, m.theme.Queued)...)
	menuView := menu.View(&m.menu, max(m.width, 1), m.theme.Plain, m.theme.Menu, m.theme.Command)
	rows := append(above, m.prompt.View())
	if menuView != "" {
		rows = append(rows, "", menuView)
	}
	body := strings.Join(rows, "\n")
	m.frameRows = lipgloss.Height(body)

	view := tea.NewView(body)
	view.KeyboardEnhancements.ReportEventTypes = true
	if position := m.prompt.Cursor(); position != nil {
		position.Y += lipgloss.Height(strings.Join(above, "\n"))
		view.Cursor = position
	}
	return view
}

func (m *Model) resize(size tea.WindowSizeMsg) tea.Cmd {
	widthChanged := size.Width != m.width
	m.width, m.windowHeight = size.Width, size.Height

	m.prompt.SetWidth(size.Width)
	m.prompt.MaxHeight = promptCap(size.Height)
	m.prompt.SetHeight(m.prompt.Height())
	m.layout(size.Height)

	if widthChanged {
		m.restyle()
	}
	return nil
}

func (m *Model) layout(height int) {
	taken := 3 + m.prompt.Height() + m.menu.Height() + m.Height(m.hist.Editing(m.Len())) + m.tasks.Rows()
	m.liveRows = layout.LiveRows(height, taken)
}

func (m *Model) printed(text string) tea.Cmd {
	budget := layout.Budget(m.windowHeight, m.frameRows)
	batches := layout.Batches(text, budget, m.width)
	var cmds []tea.Cmd
	for _, batch := range batches {
		cmds = append(cmds, tea.Println(batch))
	}
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Sequence(cmds...)
}
