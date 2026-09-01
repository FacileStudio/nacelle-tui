package main

import (
	"fmt"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle"
)

// toolGroup is one rendered row of the transcript. When grouping is on it
// represents N calls of the same tool that arrived back to back; when it is
// off it is exactly one call, and the model never builds a group at all.
//
// The grouping exists because a model that calls the same read ten times in
// a row is the common case, and ten lines of identical icon and identical
// one-line report are noise that hides the answer underneath. Grouping also
// fixes the blank-line problem: the agent emits a ToolEvent, the client
// repaints, and between the two the transcript shows a gap. A group is
// painted once when it closes, so there is no intermediate frame to leave a
// blank line sitting where the next tool's row belongs.
type toolGroup struct {
	name      string
	input     string
	count     int
	start     time.Time
	end       time.Time
	failed    bool
	discarded bool
	// the call that decided the group's fate — the one whose result or error
	// is shown, and whose input is the one rendered. Earlier calls in the
	// group are identical in shape, so their input is the same string and
	// picking the last is as good as picking any.
	tool nacelle.ToolEvent
}

// groupLine is a group as the one line it reads as: the tool's glyph and
// name, the argument that says which thing it acted on, and — when the
// group holds more than one call — how many, as `(n×)`. The duration and
// outcome fold in the same way a single call's line does, in toolline.go.
//
// A group is painted once, when it closes, so there is no intermediate
// frame to leave a blank line sitting where the next row belongs. That is
// the blank-line problem the grouping exists to fix: the agent emits a
// ToolEvent, the client repaints, and between the two the transcript shows
// a gap.
func (g toolGroup) groupLine(width int) string {
	room := width - lipgloss.Width(g.name) - durationRoom - len("• () ×n")
	inner := truncate(unstyled(primaryArg(g.input)), room)
	line := g.groupGlyph() + " " + g.name + "(" + inner + ")"
	if g.count > 1 {
		line += fmt.Sprintf(" ×%d", g.count)
	}
	return line
}

// groupGlyph returns the icon for a group, honouring the same tone map a
// single call uses. The discarded and failed states keep their own markers
// because they are different outcomes, not just more of the same.
func (g toolGroup) groupGlyph() string {
	if g.discarded {
		return "⊘"
	}
	if g.failed {
		return "✗"
	}
	return toolGlyph(g.name)
}
