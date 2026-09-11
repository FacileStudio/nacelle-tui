package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// A shrink mid-run re-wraps held rows at the new width so nothing is clipped,
// and a row that opened with a box border keeps that border on every
// continuation — the terminal reflows only lines it wrapped itself, so the
// hold has to do it here.
func TestResizeReflowsHeldRows(t *testing.T) {
	m := sized()
	m.hold = []string{"▌ " + strings.Repeat("word ", 30), strings.Repeat("x", 100), "short"}
	m.resize(tea.WindowSizeMsg{Width: 30, Height: 24})
	for _, row := range m.hold {
		if lipgloss.Width(row) > 29 {
			t.Errorf("held row survived the shrink too wide: %q", row)
		}
	}
	var bordered, plain int
	for _, row := range m.hold {
		switch {
		case strings.HasPrefix(row, "▌ "):
			bordered++
		case strings.HasPrefix(row, "x"), row == "short":
			plain++
		}
	}
	if bordered < 2 {
		t.Errorf("bordered continuations = %d, want each continuation carrying the border", bordered)
	}
	if plain == 0 {
		t.Errorf("plain continuations vanished")
	}
}
