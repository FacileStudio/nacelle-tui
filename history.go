package main

import (
	tea "charm.land/bubbletea/v2"
)

// promptHistory is the sent-question list behind the arrow keys.
//
// Up recalls the previous question into the input and each further Up walks
// further back; Down walks forward again, and one step past the newest entry
// restores whatever was being written when the walk started. The draft is
// kept for exactly that round trip and nowhere else — it is not an entry
// until it is sent, because half a thought is not history.
type promptHistory struct {
	past  []string
	index int
	draft string
}

// recall moves one entry back through the history, reporting whether it
// claimed the press. It declines only when there is nothing to recall.
func (m *model) recall() bool {
	if len(m.hist.past) == 0 {
		return false
	}
	if m.hist.index == len(m.hist.past) {
		m.hist.draft = m.prompt.Value()
	}
	m.hist.index--
	m.setEntry(m.hist.past[m.hist.index])
	return true
}

// advance moves one entry forward, restoring the draft past the newest
// entry, and reports whether it claimed the press.
func (m *model) advance() bool {
	if m.hist.index >= len(m.hist.past) {
		return false
	}
	m.hist.index++
	value := ""
	if m.hist.index < len(m.hist.past) {
		value = m.hist.past[m.hist.index]
	} else {
		value = m.hist.draft
	}
	m.setEntry(value)
	return true
}

// remember files one sent question at the end of the history and ends any
// walk through it. Duplicates are moved rather than re-appended, so pressing
// Up from the prompt offers the question actually asked last, however long
// this session has been running.
func (m *model) remember(question string) {
	for i, past := range m.hist.past {
		if past == question {
			m.hist.past = append(m.hist.past[:i], m.hist.past[i+1:]...)
			break
		}
	}
	m.hist.past = append(m.hist.past, question)
	m.hist.index = len(m.hist.past)
	m.hist.draft = ""
}

// setEntry puts one recalled line in the input with the caret at the end,
// which is where somebody pressing Up expects their next keystroke to land.
func (m *model) setEntry(value string) {
	m.prompt.Reset()
	m.prompt.SetValue(value)
	m.prompt.MoveToEnd()
}

// atFirstRow reports whether the caret sits on the prompt's first visual
// row. A wrapped question owns its other rows for cursor movement, so Up
// only reaches for the history from the top of the input — everywhere else
// it still means "caret up", the way every editor taught hands to expect.
func (m *model) atFirstRow() bool {
	position := m.prompt.Cursor()
	return position == nil || position.Y <= 0
}

// historyKey owns the arrow keys' second job: walking the sent-question
// history. Up recalls from the input's first row and Down walks back forward;
// anywhere the walk has nothing to say — no history yet, the caret mid-wrap,
// Down at the end of it — the press falls through unhandled to the textarea's
// own cursor movement.
func (m *model) historyKey(press tea.KeyPressMsg) (bool, tea.Cmd) {
	switch press.String() {
	case "up":
		return m.atFirstRow() && m.recall(), nil
	case "down":
		if m.hist.index >= len(m.hist.past) {
			return false, nil
		}
		return m.advance(), nil
	}
	return false, nil
}
