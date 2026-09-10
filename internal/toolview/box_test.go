package toolview

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Box draws one full-width pane per row: a box-drawing vertical spine in the
// caller's colour over the shared block background, then the row itself, each
// ending in a newline so a joined block gets a blank row after it.
func TestBoxRendersOneSpinePerRow(t *testing.T) {
	out := Box([]string{"one", "two"}, "2")
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("Box = %q, want a trailing newline", out)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("Box = %q, want one row per input row", out)
	}
	for _, row := range lines {
		clean := ansi.Strip(row)
		if !strings.HasPrefix(clean, "│") {
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
	if !strings.Contains(Box([]string{"x"}, "2"), "\x1b[32;48;5;237m│") {
		t.Errorf("Box(green) missed the true SGR green spine")
	}
	if !strings.Contains(Box([]string{"x"}, "1"), "\x1b[31;48;5;237m│") {
		t.Errorf("Box(red) missed the true SGR red spine")
	}
	if !strings.Contains(Box([]string{"x"}, "5"), "\x1b[35;48;5;237m│") {
		t.Errorf("Box(magenta) missed the true SGR magenta spine")
	}
	if !strings.Contains(Box([]string{"x"}, "208"), "\x1b[38;5;208;48;5;237m│") {
		t.Errorf("Box(208) missed the ANSI256 orange spine")
	}
	if strings.Contains(Box([]string{"x"}, "2"), "38;5;32") {
		t.Errorf("Box(green) leaked an ANSI256 grey-range escape")
	}
}

func TestBoxRendersNothingForNoRows(t *testing.T) {
	if got := Box(nil, "2"); got != "" {
		t.Errorf("Box(no rows) = %q, want the empty string", got)
	}
}
