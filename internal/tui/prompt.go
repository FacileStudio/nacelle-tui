package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const promptRows = 10

func newPrompt() textarea.Model {
	prompt := textarea.New()
	prompt.Placeholder = "Ask something. Esc stops a run, ctrl+c stops or quits, ctrl+\\ forces it."
	prompt.SetPromptFunc(2, continuation)
	prompt.ShowLineNumbers = false
	prompt.DynamicHeight = true
	prompt.MinHeight = 1
	prompt.MaxHeight = promptRows
	prompt.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter"))
	prompt.SetVirtualCursor(false)
	prompt.Focus()
	return prompt
}

func continuation(info textarea.PromptInfo) string {
	if info.LineNumber == 0 {
		return lipgloss.NewStyle().Bold(true).Render("| ")
	}
	return "  "
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

	// A message typed while the main run is busy answers on a fresh side
	// agent rather than sitting queued until the run — often a whole parallel
	// fan-out — finishes. A command still queues, because a control line like
	// /quit must not spawn a conversation of its own.
	if _, isCommand := m.parseCommand(question); !isCommand {
		m.startSide(question)
		return nil
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
