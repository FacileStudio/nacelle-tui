package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestAltEnterInsertsNewlineAfterPaste(t *testing.T) {
	m := bareBanner()
	m.promptRoute(tea.PasteMsg{Content: "pasted content"})
	if got := m.prompt.Value(); got != "pasted content" {
		t.Fatalf("paste = %q, want %q", got, "pasted content")
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	if got := m.prompt.Value(); got != "pasted content\n" {
		t.Errorf("alt+enter after paste = %q, want %q", got, "pasted content\n")
	}
}

func TestShiftEnterInsertsNewline(t *testing.T) {
	m := bareBanner()
	m.prompt.SetValue("first line")

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	if got := m.prompt.Value(); got != "first line\n" {
		t.Errorf("shift+enter = %q, want %q", got, "first line\n")
	}
	m.prompt.InsertString("second")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	if got := m.prompt.Value(); got != "first line\nsecond\n" {
		t.Errorf("shift+enter = %q, want %q", got, "first line\nsecond\n")
	}
}
