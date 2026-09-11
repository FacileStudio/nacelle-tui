package toolview

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Box draws one full-width pane per row: a heavy box-drawing vertical spine in
// the caller's colour over the shared block background, then the row itself,
// each ending in a newline so a joined block gets a blank row after it.
func TestBoxRendersOneSpinePerRow(t *testing.T) {
	out := Box([]string{"one", "two"}, "2", false, 80)
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("Box = %q, want a trailing newline", out)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("Box = %q, want one row per input row", out)
	}
	for _, row := range lines {
		clean := ansi.Strip(row)
		if !strings.HasPrefix(clean, "▌") {
			t.Errorf("row %q lacks the box-drawing left spine", row)
		}
		if !strings.Contains(row, "48;5;237") {
			t.Errorf("row %q lost the block background", row)
		}
	}
}

// The spine's colour is the caller's basic-palette code rendered as the plain
// SGR escape — true green and red, never the ANSI256 indices those very codes
// would be read as (desaturated teals and slates). ANSI256 codes like the MCP
// orange pass through untouched. Foreground and background combine into one
// SGR sequence, so the border row opens with it.
func TestBoxSpineColoursAreTheTrueHues(t *testing.T) {
	if !strings.Contains(Box([]string{"x"}, "2", false, 80), "\x1b[32;48;5;237m▌") {
		t.Errorf("Box(green) missed the true SGR green spine")
	}
	if !strings.Contains(Box([]string{"x"}, "1", false, 80), "\x1b[31;48;5;237m▌") {
		t.Errorf("Box(red) missed the true SGR red spine")
	}
	if !strings.Contains(Box([]string{"x"}, "5", false, 80), "\x1b[35;48;5;237m▌") {
		t.Errorf("Box(magenta) missed the true SGR magenta spine")
	}
	if !strings.Contains(Box([]string{"x"}, "208", false, 80), "\x1b[38;5;208;48;5;237m▌") {
		t.Errorf("Box(208) missed the ANSI256 orange spine")
	}
	if strings.Contains(Box([]string{"x"}, "2", false, 80), "38;5;32") {
		t.Errorf("Box(green) leaked an ANSI256 grey-range escape")
	}
}

func TestBoxRendersNothingForNoRows(t *testing.T) {
	if got := Box(nil, "2", false, 80); got != "" {
		t.Errorf("Box(no rows) = %q, want the empty string", got)
	}
}

// Transparent mode drops the grey block backdrop so the pane floats on the
// terminal's own background; the spine keeps its colour, so the box still reads
// as a pane without painting a background band of its own.
func TestBoxTransparentDropsTheBlockBackdrop(t *testing.T) {
	if strings.Contains(Box([]string{"x"}, "2", true, 80), "48;5;237") {
		t.Errorf("Box(transparent) kept the block background")
	}
	if !strings.Contains(Box([]string{"x"}, "2", true, 80), "▌") {
		t.Errorf("Box(transparent) lost the spine")
	}
}

// A row longer than the pane is wrapped by Box rather than left for the
// terminal to soft-wrap, because a terminal's continuation carries no border
// and the pane visually falls apart at that point.
func TestBoxWrapsLongRowsAndBordersTheContinuation(t *testing.T) {
	row := strings.Repeat("ab", 60)
	out := Box([]string{row}, "2", false, 40)
	for line := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		if !strings.Contains(line, "▌") {
			t.Errorf("wrapped row lost its border on a continuation: %q", ansi.Strip(line))
		}
	}
}

// WrapRow, the caller-laid-out variant, prefixes continuations of a bordered
// row with the border glyph and leaves an unbordered row's continuations bare.
func TestWrapRowPrefixesBorderOnlyOnBorderedRows(t *testing.T) {
	bordered := "▌ " + strings.Repeat("x", 50)
	pieces := WrapRow(bordered, 20)
	if len(pieces) < 2 {
		t.Fatalf("WrapRow = %d pieces, want several", len(pieces))
	}
	for _, piece := range pieces[1:] {
		if !strings.HasPrefix(piece, "▌ ") {
			t.Errorf("continuation = %q, want the border glyph", piece)
		}
	}
	plain := strings.Repeat("y", 50)
	if got := WrapRow(plain, 20); strings.HasPrefix(got[1], "▌") {
		t.Errorf("unbordered continuation = %q, want no border glyph", got[1])
	}
}
