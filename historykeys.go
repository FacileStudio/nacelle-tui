package main

import (
	tea "charm.land/bubbletea/v2"
)

// The arrow keys' own bindings, split from the walk they drive for the
// reason menukeys.go is split from menu.go: filet caps a file at eight
// functions, and a binding and the state it moves are the seam that was
// already there. history.go decides where the walk goes; this decides which
// presses ask it to.

// atFirstRow reports whether the caret sits on the prompt's first visual
// row. A wrapped question owns its other rows for cursor movement, so Up
// only reaches for the history from the top of the input — everywhere else
// it still means "caret up", the way every editor taught hands to expect.
func (m *model) atFirstRow() bool {
	position := m.prompt.Cursor()
	return position == nil || position.Y <= 0
}

// historyKey owns the arrow keys' second job: walking the sent questions and
// the queued lines behind them. Up recalls from the input's first row and Down
// walks back forward; anywhere the walk has nothing to say — nothing sent and
// nothing queued, the caret mid-wrap, Down at the end of it — the press falls
// through unhandled to the textarea's own cursor movement.
func (m *model) historyKey(press tea.KeyPressMsg) (bool, tea.Cmd) {
	switch press.String() {
	case "up":
		return m.atFirstRow() && m.recall(), nil
	case "down":
		return m.advance(), nil
	}
	return false, nil
}
