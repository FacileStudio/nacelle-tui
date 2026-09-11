package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

const promptRows = 10

// minHeightRows is the shortest the input renders even when it holds a single
// line, so the field reads as a single line rather than a padded bar. The blank
// row above the whole field is added by the view assembly.
const minHeightRows = 1

// newPrompt builds the compose textarea. The prompt has no prefix: the first
// row opens with a single margin space, so a wrapped question reads as one
// block and the text never touches the left edge. placeholder is the ghost text
// shown while the prompt is empty.
func newPrompt(placeholder string) textarea.Model {
	prompt := textarea.New()
	prompt.Placeholder = placeholder
	prompt.SetPromptFunc(1, continuation)
	prompt.ShowLineNumbers = false
	prompt.DynamicHeight = true
	prompt.MinHeight = minHeightRows
	prompt.MaxHeight = promptRows
	prompt.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter"))
	prompt.SetVirtualCursor(false)
	prompt.Focus()
	return prompt
}

// continuation is the gutter text for every row: a single margin space, so the
// first row opens one space from the left edge and wrapped rows hang under it.
func continuation(textarea.PromptInfo) string {
	return " "
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
