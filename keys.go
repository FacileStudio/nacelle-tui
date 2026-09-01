package main

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// The top-level key router, split from model.go because that file sat on
// filet's 250-line cap. The seam is the one menukeys.go and historykeys.go
// already use: model.go holds what the client is, and a file per group of
// bindings holds which press reaches what.

// key handles this client's bindings, reporting whether it consumed the
// press. Anything the scroller does not claim belongs to the prompt.
//
// Ctrl+C cancels a run in flight and only quits when nothing is running, so
// a long answer can be abandoned without losing the session — the terminal
// is in raw mode, so nothing quits on Ctrl+C unless this says so. That needs
// an escape hatch: busy only clears once settle sees the results channel
// close, and a tool wedged on a subprocess never closes it. A second ctrl+c
// inside forceQuit, or ctrl+\ at any time, quits regardless — otherwise the
// only way out of an alt-screen raw-mode terminal is kill -9 from elsewhere.
//
// Both stay live while a tool approval is pending too: a question nobody
// answers must not be a second way to get stuck. See decide's doc comment
// for why cancelling clears run.pending directly instead.
//
// The dropdown menu is checked next, ahead of both enter and the scroller:
// while it's open, up/down/tab/enter/esc belong to picking a command, not
// to scrolling the transcript or sending what's typed. Anything the menu
// itself does not claim (an ordinary character, backspace) falls all the
// way through to the prompt, which is what keeps its own filter editable.
//
// Ctrl+t sits above esc and takes the press unconditionally — see reveal for
// what it does with nothing to expand, and why that is not the call escaped
// makes. Esc reports an idle press unhandled because esc is the key every
// terminal reader uses to back out of something; nobody presses ctrl+t at a
// prompt meaning anything at all.
//
// Nobody except the textarea, which binds it to transpose-character-backward
// and now never sees it. That is the trade taken knowingly: transposing the
// two characters behind the cursor is an emacs habit almost nothing in this
// prompt is edited by, and reading the model's reasoning is a thing somebody
// wants several times a session.
//
// Esc stops a run and does nothing else, which is the whole reason it is
// worth having next to a ctrl+c that already cancels: the key that stops the
// answer is then never the key that might close the client, so there is no
// press that has to be thought about first. It sits below the menu on
// purpose — esc closes the dropdown before it stops anything, because a
// dropdown standing open is the nearer thing to back out of, and it is what
// esc already meant there. See escaped for what it does with no run to stop.
func (m *model) key(press tea.KeyPressMsg) (bool, tea.Cmd) {
	switch press.String() {
	case "ctrl+\\":
		return true, tea.Quit
	case "ctrl+c":
		if !m.run.busy || time.Since(m.run.interrupted) < forceQuit {
			return true, tea.Quit
		}
		m.abandon()
		return true, nil
	}
	if m.run.pending != nil {
		return true, m.decide(press)
	}
	if m.menu.open() {
		if handled, cmd := m.navigateMenu(press); handled {
			return true, cmd
		}
	}
	switch press.String() {
	case "ctrl+t":
		return m.reveal()
	case "esc":
		return m.escaped()
	case "tab":
		if cmd := anyCommand(m.prompt.Value()); cmd != "" {
			matches := filterMenu(m.menu.items, cmd)
			if len(matches) == 1 {
				m.prompt.SetValue(replaceCommand(m.prompt.Value(), matches[0].value))
				m.prompt.CursorEnd()
				return true, nil
			}
			if len(matches) > 1 {
				m.menu.filtered = matches
				m.menu.selected, m.menu.scroll = 0, 0
				m.layout(m.windowHeight)
				return true, nil
			}
		}
		return false, nil
	case "enter":
		return true, m.ask()
	}
	return m.historyKey(press)
}
