package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const promptRows = 10

// minHeightRows is the shortest the input renders even when it holds a single
// line, so the field reads as a single line rather than a padded bar. The blank
// row above the whole field is added by the view assembly.
const minHeightRows = 1

// newPrompt builds the compose textarea. prefix is what the first row shows
// ahead of the caret — "| " out of the box — always followed by one space of
// margin before the text, and continuation rows get a matching run of spaces so
// a wrapped question reads as one block. An empty prefix still leaves that one
// space: the text never touches the left edge.
func newPrompt(prefix string, placeholder string) textarea.Model {
	prompt := textarea.New()
	prompt.Placeholder = placeholder
	prompt.SetPromptFunc(lipgloss.Width(prefix)+1, continuation(prefix))
	prompt.ShowLineNumbers = false
	prompt.DynamicHeight = true
	prompt.MinHeight = minHeightRows
	prompt.MaxHeight = promptRows
	prompt.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter"))
	prompt.SetVirtualCursor(false)
	prompt.Focus()
	return prompt
}

// continuation is the gutter text for every row. The first row carries the
// prefix plus a trailing margin space; every later row carries an indent as wide
// as that same gutter so wrapped text hangs under what it follows. The margin
// space is always there, so even an empty prefix leaves one space before the
// text on the first row and one space of gutter on continuation rows.
func continuation(prefix string) func(textarea.PromptInfo) string {
	indent := strings.Repeat(" ", lipgloss.Width(prefix)+1)
	return func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return prefix + " "
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

	if !m.run.busy && !m.compacting {
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
