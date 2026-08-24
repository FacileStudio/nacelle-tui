package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Up from the top row of the prompt recalls the last question sent, and each
// further Up walks one entry further back.
func TestUpRecallsSentQuestionsNewestFirst(t *testing.T) {
	m := sized()
	m.remember("first question")
	m.remember("second question")

	m.key(tea.KeyPressMsg{Code: tea.KeyUp})
	if got := m.prompt.Value(); got != "second question" {
		t.Fatalf("prompt = %q, want the newest question recalled", got)
	}
	m.key(tea.KeyPressMsg{Code: tea.KeyUp})
	if got := m.prompt.Value(); got != "first question" {
		t.Fatalf("prompt = %q, want the older question next", got)
	}
}

// Down walks back forward and restores whatever was being written once past
// the newest entry.
func TestDownRestoresTheDraftPastTheNewestEntry(t *testing.T) {
	m := sized()
	m.remember("an earlier question")
	m.prompt.SetValue("half a thought")

	m.key(tea.KeyPressMsg{Code: tea.KeyUp})
	if got := m.prompt.Value(); got != "an earlier question" {
		t.Fatalf("prompt = %q, want the question recalled", got)
	}
	m.key(tea.KeyPressMsg{Code: tea.KeyDown})
	if got := m.prompt.Value(); got != "half a thought" {
		t.Fatalf("prompt = %q, want the draft restored", got)
	}
	m.key(tea.KeyPressMsg{Code: tea.KeyDown})
	if got := m.prompt.Value(); got != "half a thought" {
		t.Fatalf("prompt = %q, want the draft kept at the end of the walk", got)
	}
}

// A second send of the same question moves it to the end rather than
// duplicating it, so Up offers what was asked last.
func TestRememberMovesADuplicateToTheEnd(t *testing.T) {
	m := sized()
	m.remember("one")
	m.remember("two")
	m.remember("one")

	if got := strings.Join(m.hist.past, "|"); got != "two|one" {
		t.Fatalf("history = %q, want the duplicate moved to the end", got)
	}
	m.key(tea.KeyPressMsg{Code: tea.KeyUp})
	if got := m.prompt.Value(); got != "one" {
		t.Fatalf("prompt = %q, want the most recently resent question", got)
	}
}

// With no history behind it, Up belongs to the textarea's cursor movement
// and is not claimed by the recall path.
func TestUpWithNoHistoryIsUnhandled(t *testing.T) {
	m := sized()
	if handled, _ := m.key(tea.KeyPressMsg{Code: tea.KeyUp}); handled {
		t.Error("up was claimed with an empty history")
	}
}
