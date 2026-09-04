package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle-tui/internal/tui/menu"
)

func TestMenuItemsListsCommandsBeforeSkillsWithDescriptions(t *testing.T) {
	items := menuItems(map[string]skill{"deploy": {Name: "deploy", Description: "ships the app"}})

	if len(items) != len(commands)+1 {
		t.Fatalf("menuItems = %+v, want every command plus the one skill", items)
	}
	last := items[len(items)-1]
	if last.Value != "/skill:deploy" || last.Description != "ships the app" {
		t.Errorf("last item = %+v, want the skill, with its own description", last)
	}
	for _, it := range items[:len(items)-1] {
		if it.Description != "" {
			t.Errorf("command %q carried a description %q, want none", it.Value, it.Description)
		}
	}
}

func TestRefreshMenuClosesOnceACommandIsTypedOutInFull(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/clear")

	m.refreshMenu()

	if m.menu.Open() {
		t.Errorf("filtered = %+v, want the menu closed with nothing left to complete", m.menu.Filtered)
	}
}

func TestRefreshMenuReopensWhenTypingPastAnExactMatch(t *testing.T) {
	m := sized()
	m.menu.Items = []menu.Item{{Value: "/skill:review"}, {Value: "/skill:review-pr"}}

	m.prompt.SetValue("/skill:review")
	m.refreshMenu()
	if m.menu.Open() {
		t.Fatal("menu stayed open on an exact match")
	}

	m.prompt.SetValue("/skill:review-")
	m.refreshMenu()
	if !m.menu.Open() {
		t.Error("menu did not come back once the line no longer named a candidate outright")
	}
}

func TestKeyEnterSendsAFullyTypedCommandInsteadOfRepickingIt(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/clear")
	m.refreshMenu()

	m.key(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := m.prompt.Value(); got != "" {
		t.Errorf("prompt = %q after enter, want the command sent rather than picked again", got)
	}
}

func TestRefreshMenuOpensOnASlashWithNoQuery(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()

	if !m.menu.Open() {
		t.Fatal("menu did not open on a bare '/'")
	}
	if len(m.menu.Filtered) != len(commands) {
		t.Errorf("filtered = %+v, want every command with no query yet", m.menu.Filtered)
	}
}

func TestRefreshMenuClosesAndForgetsDismissalOnceTheSlashIsGone(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()
	m.menu.Dismissed = true

	m.prompt.SetValue("hello")
	m.refreshMenu()

	if m.menu.Open() {
		t.Error("menu stayed open once the line no longer started with '/'")
	}
	if m.menu.Dismissed {
		t.Error("dismissed survived the line that cleared it")
	}
}

func TestRefreshMenuNarrowsAsMoreIsTyped(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/cl")
	m.refreshMenu()

	if len(m.menu.Filtered) != 1 || m.menu.Filtered[0].Value != "/clear" {
		t.Errorf("filtered = %+v, want exactly /clear", m.menu.Filtered)
	}
}

func TestNavigateMenuMovesSelectionWithinBounds(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()

	m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.menu.Selected != 0 {
		t.Errorf("selected = %d after up at the top, want it to stay at 0", m.menu.Selected)
	}

	for range m.menu.Filtered {
		m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.menu.Selected != len(m.menu.Filtered)-1 {
		t.Errorf("selected = %d after running past the bottom, want it to stop at %d", m.menu.Selected, len(m.menu.Filtered)-1)
	}
}

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
	if m.menu.Open() {
		t.Error("the menu stayed open after a selection")
	}
}

func TestNavigateMenuEscDismissesWithoutChangingText(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/cl")
	m.refreshMenu()

	handled, _ := m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyEscape})

	if !handled {
		t.Fatal("navigateMenu did not claim esc while the menu was open")
	}
	if got := m.prompt.Value(); got != "/cl" {
		t.Errorf("prompt = %q, want esc to leave the typed text alone", got)
	}
	if m.menu.Open() {
		t.Error("the menu stayed open after esc")
	}
}

func TestViewMenuIsEmptyWhenClosed(t *testing.T) {
	m := sized()
	if got := m.viewMenu(); got != "" {
		t.Errorf("viewMenu() = %q, want empty with nothing typed", got)
	}
}
