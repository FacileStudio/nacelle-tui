package tui

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
