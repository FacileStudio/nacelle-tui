package diff

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	diffAdded   = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(2))
	diffRemoved = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(1))
)

func truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	return ansi.Truncate(s, limit, "…")
}

// renderBlock writes one hunk's lines, stopping at the display cap with a
// marker saying the diff was cut rather than pretending it wasn't. It
// returns how many lines are through and whether the cap was reached.
func renderBlock(out *strings.Builder, block []diffOp, width, shown int, muted lipgloss.Style) (int, bool) {
	for _, op := range block {
		if shown >= shownDiffLines {
			out.WriteString(muted.Render("  … more"))
			return shown, true
		}
		out.WriteString(renderOp(op, width, muted))
		shown++
	}
	return shown, false
}

// renderOp is one diff line as styled text, indented past the ⏺ that names
// the call above it and cut to the window so nothing wraps.
func renderOp(op diffOp, width int, muted lipgloss.Style) string {
	style, prefix := muted, "    "
	switch op.kind {
	case '-':
		style, prefix = diffRemoved, "  - "
	case '+':
		style, prefix = diffAdded, "  + "
	}
	return style.Render(prefix+truncate(op.text, max(width-lipgloss.Width(prefix), 1))) + "\n"
}

// splitLines breaks file contents into diffable lines, dropping a single
// trailing newline — the marker of a well-formed text file, not a line of it.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// RenderDiff draws one change the way git would: removals in the terminal's
// red, additions in its green, and a few unchanged lines around each block so
// the eye can find where in the file it is looking.
//
// The colours are ANSI indices rather than fixed values, so they follow
// whatever scheme the terminal itself uses. Nothing worth showing — a call
// whose input could not be parsed, a change that touches no line — renders as
// empty, and the caller simply says the ordinary one-line report it always
// has.
func RenderDiff(change EditChange, width int, muted lipgloss.Style) string {
	if change.Path == "" || change.Before == change.After {
		return ""
	}
	blocks := hunks(diffOps(splitLines(change.Before), splitLines(change.After)), contextLines)
	if len(blocks) == 0 {
		return ""
	}

	var out strings.Builder
	shown := 0
	for i, block := range blocks {
		if i > 0 {
			out.WriteString(muted.Render("  …") + "\n")
			shown++
		}
		var cut bool
		shown, cut = renderBlock(&out, block, width, shown, muted)
		if cut {
			break
		}
	}
	return out.String()
}
