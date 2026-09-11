package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestPromptBackdropCoversEveryRow: a two-line question — the first line away
// from the cursor, the second wrapping over the edge — must carry the backdrop
// on every row. The textarea only backdrops the cursor's own logical line, so
// without the base backdrop the rows off the cursor fall back to the terminal
// default and the prompt reads as a patchwork.
func TestPromptBackdropCoversEveryRow(t *testing.T) {
	m := sized()
	m.prompt.SetValue("first line\nsecond line that is quite long and wraps over onto a further row, going past the edge")
	m.resize(tea.WindowSizeMsg{Width: 60, Height: 24})

	rows := strings.Split(m.prompt.View(), "\n")
	if got := len(rows); got != 3 {
		t.Fatalf("prompt drew %d rows, want the two lines across three", got)
	}
	for i, row := range rows {
		if !strings.Contains(row, "\x1b[40m") {
			t.Errorf("row %d = %q, want the backdrop on every row", i, row)
		}
		if got := visible(row); !strings.Contains(got, "▌") {
			t.Errorf("row %d = %q, want the muted ▌ gutter on every row", i, row)
		}
	}
}
