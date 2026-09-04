package tui

func (m *Model) walkable() []string {
	return m.hist.Walkable(m.run.queued)
}

func (m *Model) recall() bool {
	text, ok := m.hist.Recall(m.prompt.Value(), m.run.queued)
	if !ok {
		return false
	}
	m.setEntry(text)
	return true
}

func (m *Model) advance() bool {
	text, ok := m.hist.Advance(m.run.queued)
	if !ok {
		return false
	}
	m.setEntry(text)
	return true
}

func (m *Model) requeue(text string) bool {
	return m.hist.Requeue(m.run.queued, text)
}

func (m *Model) remember(question string) {
	m.hist.Remember(question, m.run.queued)
}

func (m *Model) setEntry(value string) {
	m.prompt.Reset()
	m.prompt.SetValue(value)
	m.prompt.MoveToEnd()
}
