package tui

import (
	"testing"
)

func TestSanitizePasteRemovesBracketedPasteWrapping(t *testing.T) {
	got := sanitizePaste("\x1b[200~hello world\x1b[201~")
	if got != "hello world" {
		t.Errorf("sanitizePaste = %q, want %q", got, "hello world")
	}
}

func TestSanitizePasteReplacesWindowsLineEndings(t *testing.T) {
	got := sanitizePaste("line1\r\nline2\r\n")
	if got != "line1\nline2\n" {
		t.Errorf("sanitizePaste = %q, want %q", got, "line1\nline2\n")
	}
}

func TestSanitizePasteStripsEmbeddedControlSequences(t *testing.T) {
	got := sanitizePaste("before\x1b[5;20Hafter")
	if got != "beforeafter" {
		t.Errorf("sanitizePaste = %q, want %q", got, "beforeafter")
	}
}

func TestSanitizePastePreservesRegularCharacters(t *testing.T) {
	input := "normal text with émojis 😀 and unicode №123"
	got := sanitizePaste(input)
	if got != input {
		t.Errorf("sanitizePaste = %q, want %q", got, input)
	}
}

func TestSanitizePasteStripsTabs(t *testing.T) {
	got := sanitizePaste("hello\tworld")
	if got != "hello world" {
		t.Errorf("sanitizePaste = %q, want %q", got, "hello world")
	}
}

func TestSanitizePasteHandlesEmptyInput(t *testing.T) {
	if got := sanitizePaste(""); got != "" {
		t.Errorf("sanitizePaste = %q, want empty string", got)
	}
}

func TestSanitizePasteHandlesOnlyWrapping(t *testing.T) {
	got := sanitizePaste("\x1b[200~\x1b[201~")
	if got != "" {
		t.Errorf("sanitizePaste = %q, want empty string", got)
	}
}
