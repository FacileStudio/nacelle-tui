package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) stamp() {
	if m.Begun.IsZero() && m.run.reasoning.Len() > 0 {
		m.Stamp()
	}
}

func (m *Model) thought() {
	m.Thought()
}

func (m *Model) elapsed() time.Duration {
	return m.Elapsed()
}

func (m *Model) forget() {
	m.Forget()
}

func (m *Model) collapsed(spent time.Duration) string {
	return m.Collapsed(spent)
}

func (m *Model) reveal() (bool, tea.Cmd) {
	m.Expanded = !m.Expanded
	m.Hinted = true

	switch {
	case m.Expanded && m.Retained != "":
		m.say(fromThinking, m.Retained)
	case m.Expanded:
		m.say(fromClient, "reasoning will be shown in full from here")
	default:
		m.say(fromClient, "reasoning will collapse to a single line from here")
	}
	return true, nil
}

func (m *Model) flush() string {
	m.flushThinking()

	full := m.run.fullAnswer.String()
	unprinted := ""
	if m.run.committedLen < len(full) {
		unprinted = full[m.run.committedLen:]
	}
	m.run.answer.Reset()
	m.run.fullAnswer.Reset()
	m.run.committedLen = 0
	if unprinted != "" {
		m.say(fromModel, unprinted)
	}
	return full
}
