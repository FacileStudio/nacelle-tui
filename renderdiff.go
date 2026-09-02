package main

import (
	"strings"

	"charm.land/lipgloss/v2"
)

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
