package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const promptRows = 10

// newPrompt builds the compose textarea. prefix is what the first row shows
// ahead of the caret — "| " out of the box — and continuation rows get a matching
// run of spaces so a wrapped question reads as one block. An empty prefix draws
// nothing: the first row opens at the margin and continuation rows get no indent.
func newPrompt(prefix string, placeholder string) textarea.Model {
	prompt := textarea.New()
	prompt.Placeholder = placeholder
	prompt.SetPromptFunc(lipgloss.Width(prefix), continuation(prefix))
	prompt.ShowLineNumbers = false
	prompt.DynamicHeight = true
	prompt.MinHeight = 1
	prompt.MaxHeight = promptRows
	prompt.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter"))
	prompt.SetVirtualCursor(false)
	prompt.Focus()
	return prompt
}

// continuation is the gutter text for every row. The first row carries the
// prefix itself; every later row carries an indent as wide as the prefix so
// wrapped text hangs under what it follows. An empty prefix means no gutter at
// all, so nothing is drawn on the first row and no extra spaces are added.
func continuation(prefix string) func(textarea.PromptInfo) string {
	indent := strings.Repeat(" ", lipgloss.Width(prefix))
	return func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return prefix
		}
		return indent
	}
}

func (m *Model) ask() tea.Cmd {
	question := strings.TrimSpace(m.prompt.Value())
	if question == "" {
		return nil
	}
	m.prompt.Reset()

	if !m.run.busy {
		m.hist.Remember(question, m.Items())
		m.layout(m.windowHeight)
		return m.dispatch(question)
	}

	held := m.hist.Requeue(m.Items(), question)
	if !held {
		m.Add(question)
	}
	m.hist.Remember(question, m.Items())
	m.layout(m.windowHeight)
	return nil
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
