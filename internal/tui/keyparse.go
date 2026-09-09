package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle-tui/internal/approval"
	"github.com/FacileStudio/nacelle-tui/internal/menu"
)

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
func (m *Model) key(press tea.KeyPressMsg) (bool, tea.Cmd) {
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
		m.decide(press)
		return true, nil
	}
	if m.menu.Open() {
		if m.navigateMenu(press) {
			return true, nil
		}
	}
	return m.promptKey(press)
}

// promptKey handles the keys that act on the prompt's own text, passed through
// once nothing above claims the press. Anything it does not bind falls through
// to history navigation.
func (m *Model) promptKey(press tea.KeyPressMsg) (bool, tea.Cmd) {
	switch press.String() {
	case "ctrl+t":
		return m.reveal()
	case "esc":
		return m.escaped()
	case "tab":
		return m.tabKey(), nil
	case "alt+enter":
		return false, nil
	case "enter":
		return true, m.ask()
	}
	return m.historyKey(press)
}

func (m *Model) tabKey() bool {
	cmd := m.commandFilter()
	if cmd == "" {
		return false
	}
	matches := menu.FilterMenu(m.menu.Items, cmd)
	m.menu.Reset()
	if len(matches) == 1 {
		m.prompt.SetValue(m.replaceCursorWord(matches[0].Value))
		m.prompt.CursorEnd()
		return true
	}
	if len(matches) > 1 {
		m.menu.Filtered = matches
		m.layout(m.windowHeight)
		return true
	}
	return false
}

// commandFilter returns the slash command the autocomplete should filter on:
// the slash word under the cursor when there is one, else the first /command
// in the line. Basing it on the cursor is what lets a second /command typed
// after an already-completed skill (say `/facile-review /mus`) complete on
// its own, instead of always re-matching the skill at the front of the line.
// The fallback keeps typing straight past a command (`run /cl now`) from
// closing the dropdown, mirroring the old first-slash behaviour.
func (m *Model) commandFilter() string {
	w := m.cursorWordAt()
	if strings.HasPrefix(w.text, "/") {
		return w.text
	}
	return menu.AnyCommand(m.prompt.Value())
}

// cursorWord is the word under the prompt cursor, bounded by the menu's word
// separators (space, tab, newline), with the geometry the slash handling needs.
type cursorWord struct {
	text  string
	row   int
	start int // start offset within the row
	end   int // end offset within the row, exclusive
	full  int // start offset within the whole prompt
}

// cursorWordAt returns the word under the prompt's cursor, or an empty text
// when the cursor sits on a separator or outside the buffer. full is the
// word's start offset in the whole prompt, which is what lets callers tell a
// command typed at the very start of the line from a mid-sentence one.
func (m *Model) cursorWordAt() cursorWord {
	v := m.prompt.Value()
	lines := strings.Split(v, "\n")
	row := m.prompt.Line()
	res := cursorWord{text: "", row: row, start: 0, end: 0, full: 0}
	if row < 0 || row >= len(lines) {
		return res
	}
	line := lines[row]
	idx := m.prompt.Column() - 1
	if idx < 0 || idx >= len(line) || wordGap(line[idx]) {
		return res
	}
	start := idx
	for start > 0 && !wordGap(line[start-1]) {
		start--
	}
	end := idx
	for end < len(line) && !wordGap(line[end]) {
		end++
	}
	full := start
	for i := 0; i < row; i++ {
		full += len(lines[i]) + 1
	}
	return cursorWord{text: line[start:end], row: row, start: start, end: end, full: full}
}

// replaceCursorWord rebuilds the prompt with pick in place of the slash word
// the cursor sits on, plus a trailing space, preserving the text around it.
// When the cursor is not on a slash word it falls back to replacing the first
// /command, so text typed past a command still completes in place.
func (m *Model) replaceCursorWord(pick string) string {
	w := m.cursorWordAt()
	if !strings.HasPrefix(w.text, "/") {
		return menu.ReplaceCommand(m.prompt.Value(), pick)
	}
	lines := strings.Split(m.prompt.Value(), "\n")
	lines[w.row] = lines[w.row][:w.start] + pick + " " + lines[w.row][w.end:]
	return strings.Join(lines, "\n")
}

// wordGap reports whether r separates words, mirroring the delimiter set the
// menu uses to split a /command from the text around it: space, tab, newline.
func wordGap(r byte) bool {
	return r == 0x20 || r == 0x09 || r == 0x0a
}

func (m *Model) decide(press tea.KeyPressMsg) {
	var decision approval.Decision
	switch press.String() {
	case "y":
		decision = approval.AllowedOnce
	case "a":
		decision = approval.AllowedForSession
	case "n":
		decision = approval.Denied
	default:
		return
	}

	pending := m.run.pending
	m.run.pending = nil
	pending.Decision <- decision
}

func (m *Model) refreshMenu() {
	m.prompt.SetStyles(m.promptStyles)
	w := m.cursorWordAt()
	if strings.HasPrefix(w.text, "/") && w.full == 0 {
		m.menu.Filter(w.text)
	} else {
		m.menu.Reset()
	}
	m.layout(m.windowHeight)
}

func (m *Model) navigateMenu(press tea.KeyPressMsg) bool {
	switch press.String() {
	case "up":
		m.menu.Up()
	case "down":
		m.menu.Down()
	case "tab", "enter":
		if it, ok := m.menu.SelectedItem(); ok {
			m.prompt.SetValue(m.replaceCursorWord(it.Value))
			m.prompt.CursorEnd()
			m.menu.Dismiss()
		}
	case "esc":
		m.menu.Dismiss()
	default:
		return false
	}
	m.menu.ClampView()
	m.layout(m.windowHeight)
	return true
}

func (m *Model) historyKey(press tea.KeyPressMsg) (bool, tea.Cmd) {
	switch press.String() {
	case "up":
		if pos := m.prompt.Cursor(); pos != nil && pos.Y > 0 {
			return false, nil
		}
		if text, ok := m.hist.Recall(m.prompt.Value(), m.Items()); ok {
			m.prompt.Reset()
			m.prompt.SetValue(text)
			m.prompt.MoveToEnd()
			return true, nil
		}
	case "down":
		if text, ok := m.hist.Advance(m.Items()); ok {
			m.prompt.Reset()
			m.prompt.SetValue(text)
			m.prompt.MoveToEnd()
			return true, nil
		}
	}
	return false, nil
}

func (m *Model) viewMenu() string {
	return menu.View(&m.menu, max(m.width, 1), m.theme.Plain, m.theme.Menu, m.theme.Command)
}
