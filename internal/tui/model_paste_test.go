package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Pasted content arrives as one message, so a multi-line paste must keep its
// newlines rather than being collapsed onto a single line.
func TestHandlePastePreservesMultilineContent(t *testing.T) {
	m := bareBanner()
	m.prompt.InsertString("initial")
	m.promptRoute(tea.PasteMsg{Content: "line1\nline2\nline3"})
	if got := m.prompt.Value(); got != "initialline1\nline2\nline3" {
		t.Errorf("handlePaste = %q, want %q", got, "initialline1\nline2\nline3")
	}
}

// Windows line endings are normalized to Unix style before being inserted.
func TestHandlePasteNormalizesWindowsLineEndings(t *testing.T) {
	m := bareBanner()
	m.promptRoute(tea.PasteMsg{Content: "line1\r\nline2"})
	if got := m.prompt.Value(); got != "line1\nline2" {
		t.Errorf("handlePaste = %q, want %q", got, "line1\nline2")
	}
}

// Old Mac line endings are normalized to Unix style before being inserted.
func TestHandlePasteNormalizesMacLineEndings(t *testing.T) {
	m := bareBanner()
	m.promptRoute(tea.PasteMsg{Content: "line1\rline2"})
	if got := m.prompt.Value(); got != "line1\nline2" {
		t.Errorf("handlePaste = %q, want %q", got, "line1\nline2")
	}
}
