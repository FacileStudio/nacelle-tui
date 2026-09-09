package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestAltEnterInsertsNewlineAfterPaste(t *testing.T) {
	m := bareBanner()
	m.handlePaste(tea.PasteMsg{Content: "pasted content"})
	if got := m.prompt.Value(); got != "pasted content" {
		t.Fatalf("paste = %q, want %q", got, "pasted content")
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	if got := m.prompt.Value(); got != "pasted content\n" {
		t.Errorf("alt+enter after paste = %q, want %q", got, "pasted content\n")
	}
}
