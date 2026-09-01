package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// commandWord is the text after the '/' at the start of the prompt, up to
// the first whitespace. It is the query the dropdown filters on. Only the
// start-of-line position triggers the menu, so typing "/clear" mid-sentence
// never opens it.
func commandWord(value string) string {
	if !strings.HasPrefix(value, "/") {
		return ""
	}
	rest := value[1:]
	if i := strings.IndexAny(rest, " \t\n"); i >= 0 {
		rest = rest[:i]
	}
	return "/" + rest
}

// anyCommand returns the first /command found anywhere in value, or ""
// when there is none. Used for highlighting — a /command mid-sentence is
// styled the same way as one at the start of the line, even though only
// a start-of-line / opens the dropdown menu.
func anyCommand(value string) string {
	idx := strings.Index(value, "/")
	if idx < 0 {
		return ""
	}
	rest := value[idx+1:]
	if i := strings.IndexAny(rest, " \t\n"); i >= 0 {
		rest = rest[:i]
	}
	return "/" + rest
}

// refreshMenu recomputes what the dropdown shows from the prompt's current
// value. The dropdown is the one signal that the user is in command mode, so
// the textarea itself keeps its normal appearance — applying a command colour
// to the whole widget would recolor the entire line on any mid-sentence "/".
//
// layout() runs here too, not only from resize(): the dropdown opening,
// closing, or simply matching a different number of items changes how much
// height the transcript gets, and no WindowSizeMsg arrives to trigger that
// on its own.
func (m *model) refreshMenu() {
	word := commandWord(m.prompt.Value())
	m.prompt.SetStyles(m.promptStyles)
	if word == "" {
		m.menu.filtered = nil
		m.menu.dismissed = false
	} else {
		m.menu.filtered = filterMenu(m.menu.items, word)
		if typedOut(m.menu.filtered, word) {
			m.menu.filtered = nil
		}
		if m.menu.selected >= len(m.menu.filtered) {
			m.menu.selected = 0
		}
		m.menu.clampView()
	}
	m.layout(m.windowHeight)
}

// navigateMenu handles a keypress while the dropdown is open, reporting
// whether it consumed the press — everything it does not claim (typing a
// character, backspace) falls through to the prompt as normal, which is
// what keeps the filter itself editable while the menu is up.
//
// tab/enter and esc both close the dropdown, which changes menu.height() —
// and unlike an ordinary character, neither one reaches prompt.Update, the
// path that would otherwise call refreshMenu and its layout() for free. Skip
// layout() here and the viewport stays sized for the dropdown that just
// closed, short of the terminal's real bottom, which is exactly what made
// the prompt look like it "jumped up" after picking an item.
func (m *model) navigateMenu(press tea.KeyPressMsg) (bool, tea.Cmd) {
	switch press.String() {
	case "up":
		m.menu.selected = max(m.menu.selected-1, 0)
	case "down":
		m.menu.selected = min(m.menu.selected+1, len(m.menu.filtered)-1)
	case "tab", "enter":
		m.selectMenuItem()
	case "esc":
		m.menu.dismissed = true
	default:
		return false, nil
	}
	m.menu.clampView()
	m.layout(m.windowHeight)
	return true, nil
}

// selectMenuItem fills the prompt with the highlighted entry, replacing the
// word after the last '/' in the prompt so a command selected mid-message
// leaves the rest of the sentence intact. A trailing space is added when the
// selected command is at the end of the line, ready for an argument, and
// omitted when there is already text after it.
//
// It does not submit — a /skill:name most often takes an argument, and enter
// inside the dropdown has to mean "pick this," not "send it with nothing
// typed after it yet." A second, ordinary enter — once there is nothing left
// to pick from — is what actually starts the run.
func (m *model) selectMenuItem() {
	if m.menu.selected >= len(m.menu.filtered) {
		return
	}
	m.prompt.SetValue(insertPick(m.prompt.Value(), m.menu.filtered[m.menu.selected].value))
	m.prompt.CursorEnd()
	m.menu.dismissed = true
}

// insertPick replaces the whole prompt value with the picked command,
// because the menu only opens when the prompt starts with '/'.
// The trailing space is what distinguishes a picked command from a typed one —
// enter being pressed right after the pick finishes the command, and an extra
// space awaits an argument.
func insertPick(value, pick string) string {
	return pick + " "
}

// replaceCommand replaces only the /command portion of value with the picked
// completion, preserving text before and after it. Unlike insertPick, this
// handles mid-sentence completion where the rest of the line must stay intact.
func replaceCommand(value, pick string) string {
	idx := strings.Index(value, "/")
	if idx < 0 {
		return value
	}
	rest := value[idx+1:]
	end := strings.IndexAny(rest, " \t\n")
	if end < 0 {
		end = len(rest)
	}
	return value[:idx] + pick + " " + rest[end:]
}

// viewMenu draws the dropdown, or "" when it has nothing to show — every
// keystroke that does not start a line with '/' would otherwise insert a
// blank line into View() for no reason.
//
// The selected row carries an arrow marker and the bold question style,
// unpadded: that style carries Padding(0, 1) for its other job, the
// transcript's quoted-question pill, and left as-is here it shifted only
// the selected row's text one column right of every unselected row's, which
// render through theme.plain and have no padding at all.
func (m *model) viewMenu() string {
	if !m.menu.open() {
		return ""
	}
	// The window is filtered's slice, offset by scroll — the rows actually
	// drawn are never the first few of the list, they are the ones around
	// where the selection is, and the index compared against selected is the
	// position inside that window.
	items := m.menu.filtered[m.menu.scroll : m.menu.scroll+m.menu.height()]
	width := max(m.width, 1)

	rows := make([]string, len(items))
	for i, it := range items {
		style := m.theme.plain
		marker := "  "
		if m.menu.scroll+i == m.menu.selected {
			style = m.theme.menu
			marker = "→ "
		}
		rows[i] = style.Width(width).Render(marker + menuRow(it, width-2, m.theme.command))
	}
	return strings.Join(rows, "\n")
}

// truncationSuffix is what truncate (approve.go) appends when it actually
// cuts something — 3 bytes, not 1, since "…" is multi-byte UTF-8. Named
// here so menuRow's own budget math reads as what it is instead of a bare
// "- 3" nobody can trace back to why.
const truncationSuffix = "…"

// menuRow is one dropdown line, fit to width: the value always shows in
// full — truncating a command or skill's own name would make it useless to
// read — and the description, only when there is room left worth using for
// one, fills the rest. A row this does not fit inside width, on a narrow
// enough terminal, is one line too many everywhere else in this file
// assumes it is not: height(), layout() and the cursor math in View() all
// count one line per item.
func menuRow(it menuItem, width int, cmdStyle lipgloss.Style) string {
	value := it.value
	if strings.HasPrefix(value, "/") {
		value = cmdStyle.Render(value)
	}
	if it.description == "" {
		return value
	}
	const separator = "  "
	budget := width - lipgloss.Width(value) - len(separator) - len(truncationSuffix)
	if budget < 10 {
		return value
	}
	return value + separator + truncate(it.description, budget)
}
