package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *Model) recall() bool {
	text, ok := m.hist.Recall(m.prompt.Value(), m.Items())
	if !ok {
		return false
	}
	m.setEntry(text)
	return true
}

func (m *Model) advance() bool {
	text, ok := m.hist.Advance(m.Items())
	if !ok {
		return false
	}
	m.setEntry(text)
	return true
}

func (m *Model) requeue(text string) bool {
	return m.hist.Requeue(m.Items(), text)
}

func (m *Model) remember(question string) {
	m.hist.Remember(question, m.Items())
}

func (m *Model) editing() int {
	return m.hist.Editing(m.Len())
}

func (m *Model) setEntry(value string) {
	m.prompt.Reset()
	m.prompt.SetValue(value)
	m.prompt.MoveToEnd()
}

func (m *Model) atFirstRow() bool {
	position := m.prompt.Cursor()
	return position == nil || position.Y <= 0
}

func (m *Model) historyKey(press tea.KeyPressMsg) (bool, tea.Cmd) {
	switch press.String() {
	case "up":
		return m.atFirstRow() && m.recall(), nil
	case "down":
		return m.advance(), nil
	}
	return false, nil
}
