package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Typing '/' alone is what opens the dropdown at all — with no query yet,
// refreshMenu has to show the full pool, not wait for a second character.
func TestRefreshMenuOpensOnASlashWithNoQuery(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")

	m.refreshMenu()

	if !m.menu.open() {
		t.Fatal("menu did not open on a bare '/'")
	}
	if len(m.menu.filtered) != len(commands) {
		t.Errorf("filtered = %+v, want every command with no query yet", m.menu.filtered)
	}
}

// Deleting back past the '/' has to close the menu and forget any earlier
// esc — otherwise a dismissed dropdown stays dismissed for a line that has
// not even been typed yet.
func TestRefreshMenuClosesAndForgetsDismissalOnceTheSlashIsGone(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()
	m.menu.dismissed = true

	m.prompt.SetValue("hello")
	m.refreshMenu()

	if m.menu.open() {
		t.Error("menu stayed open once the line no longer started with '/'")
	}
	if m.menu.dismissed {
		t.Error("dismissed survived the line that cleared it")
	}
}

func TestRefreshMenuNarrowsAsMoreIsTyped(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/cl")

	m.refreshMenu()

	if len(m.menu.filtered) != 1 || m.menu.filtered[0].value != "/clear" {
		t.Errorf("filtered = %+v, want exactly /clear", m.menu.filtered)
	}
}

func TestNavigateMenuMovesSelectionWithinBounds(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()

	m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.menu.selected != 0 {
		t.Errorf("selected = %d after up at the top, want it to stay at 0", m.menu.selected)
	}

	for range m.menu.filtered {
		m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.menu.selected != len(m.menu.filtered)-1 {
		t.Errorf("selected = %d after running past the bottom, want it to stop at %d", m.menu.selected, len(m.menu.filtered)-1)
	}
}

// tab picking a command has to leave the run alone — the whole reason enter
// inside the dropdown does not double as ask()'s enter.
func TestNavigateMenuTabSelectsWithoutStartingARun(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/cl")
	m.refreshMenu()

	handled, _ := m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyTab})

	if !handled {
		t.Fatal("navigateMenu did not claim tab while the menu was open")
	}
	if got := m.prompt.Value(); got != "/clear " {
		t.Errorf("prompt = %q, want the picked command plus a trailing space", got)
	}
	if m.run.busy {
		t.Error("selecting from the menu started a run")
	}
	if m.menu.open() {
		t.Error("the menu stayed open after a selection")
	}
}

// Live-driving this turned up the same bug tab and esc share: closing the
// dropdown without re-running layout() left the live region sized for the menu
// that just disappeared, so the prompt rendered short of the real bottom of
// the terminal — it looked like it had "jumped up" to where the dropdown
// used to be.
func TestNavigateMenuTabRestoresViewportHeightAfterClosingTheMenu(t *testing.T) {
	m := sized()
	before := m.liveRows
	m.prompt.SetValue("/cl")
	m.refreshMenu()
	if m.liveRows >= before {
		t.Fatalf("live rows = %d, want it shrunk below %d while the menu is open", m.liveRows, before)
	}

	m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyTab})

	if got := m.liveRows; got != before {
		t.Errorf("live rows = %d after selecting, want it restored to %d now the menu is closed", got, before)
	}
}

func TestNavigateMenuEscDismissesWithoutChangingText(t *testing.T) {
	m := sized()
	before := m.liveRows
	m.prompt.SetValue("/cl")
	m.refreshMenu()

	handled, _ := m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyEscape})

	if !handled {
		t.Fatal("navigateMenu did not claim esc while the menu was open")
	}
	if got := m.prompt.Value(); got != "/cl" {
		t.Errorf("prompt = %q, want esc to leave the typed text alone", got)
	}
	if m.menu.open() {
		t.Error("the menu stayed open after esc")
	}
	if got := m.liveRows; got != before {
		t.Errorf("live rows = %d after esc, want it restored to %d now the menu is closed", got, before)
	}
}

// An ordinary character is exactly what navigateMenu must not claim — it is
// the prompt's own filter that has to see it, by falling all the way
// through key() to prompt.Update.
func TestNavigateMenuDoesNotClaimAnOrdinaryCharacter(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/cl")
	m.refreshMenu()

	handled, _ := m.navigateMenu(tea.KeyPressMsg{Code: 'e'})

	if handled {
		t.Error("navigateMenu claimed an ordinary character")
	}
}

// The exact bug live-verifying this turned up: a row built without regard
// to the terminal's real width wrapped in tmux's 63-column pane, which
// broke every line-counting assumption elsewhere in this file at once.
func TestMenuRowNeverExceedsWidth(t *testing.T) {
	it := menuItem{value: "/skill:antenne", description: strings.Repeat("x", 200)}

	for _, width := range []int{20, 40, 63, 80} {
		if got := menuRow(it, width, lipgloss.NewStyle()); len(got) > width {
			t.Errorf("menuRow(width=%d) = %q (%d chars), want it to fit", width, got, len(got))
		}
	}
}

func TestMenuRowDropsTheDescriptionRatherThanOverflowOnATinyWidth(t *testing.T) {
	it := menuItem{value: "/skill:facile-review", description: "a description"}

	if got := menuRow(it, 10, lipgloss.NewStyle()); got != it.value {
		t.Errorf("menuRow(width=10) = %q, want just the value with no room for a description", got)
	}
}

func TestViewMenuIsEmptyWhenClosed(t *testing.T) {
	m := sized()
	if got := m.viewMenu(); got != "" {
		t.Errorf("viewMenu() = %q, want empty with nothing typed", got)
	}
}

// replaceCommand preserves text before and after the /command, keeping
// "please run /cl" → "please run /clear " instead of losing the first half.
func TestReplaceCommandPreservesSurroundingText(t *testing.T) {
	want := "please run /clear  now"
	if got := replaceCommand("please run /cl now", "/clear"); got != want {
		t.Errorf("replaceCommand = %q, want %q", got, want)
	}
}

// With nothing before the /command, replaceCommand acts like insertPick.
func TestReplaceCommandFromStartOfLine(t *testing.T) {
	want := "/clear  now"
	if got := replaceCommand("/cl now", "/clear"); got != want {
		t.Errorf("replaceCommand = %q, want %q", got, want)
	}
}

// With no text after the command, the trailing space still appears.
func TestReplaceCommandCommandAtEndOfValue(t *testing.T) {
	want := "please run /clear "
	if got := replaceCommand("please run /cl", "/clear"); got != want {
		t.Errorf("replaceCommand = %q, want %q", got, want)
	}
}

// With no slash in value, replaceCommand returns the value unchanged.
func TestReplaceCommandWithNoSlash(t *testing.T) {
	want := "hello world"
	if got := replaceCommand(want, "/clear"); got != want {
		t.Errorf("replaceCommand(%q) = %q, want it unchanged", want, got)
	}
}
