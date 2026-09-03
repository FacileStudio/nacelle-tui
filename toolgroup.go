package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle"
)

// toolGroup is one rendered row of the transcript. When grouping is on it
// represents N calls of the same tool kind that arrived back to back; when it
// is off it is exactly one call, and the model never builds a group at all.
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
	// callNames tracks individual primary arguments in a kind-based batch for rendering.
	// Each entry is the human-readable one-line summary of that call's input
	// (the same string toolLine renders), so a batch of four shell commands reads
	// as "⏺ 4 commands · ls docs · read_file(view.go) · git status · …" rather than
	// repeating the tool name four times.
	callNames []string
	// the call that decided the group's fate — the one whose result or error
	// is shown, and whose input is the one rendered. Earlier calls in the
	// group are identical in shape, so their input is the same string and
	// picking the last is as good as picking any.
	tool nacelle.ToolEvent
	// finishedCount tracks how many calls in this group have completed.
	// The group line is printed only when this reaches count, preventing
	// duplicate lines when each tool result in a batch triggers finished().
	finishedCount int
}

// groupLine is a group as the one line it reads as. For a single call, it's
// the tool's glyph, name, and primary argument. For a batch of same-kind calls,
// it shows the kind glyph and a compact summary like "4 commands · cmd1 · cmd2 · …".
// The duration and outcome fold in the same way a single call's line does.
//
// A group is painted once, when it closes, so there is no intermediate
// frame to leave a blank line sitting where the next row belongs.
func (g toolGroup) groupLine(width int) string {
	if g.count <= 1 {
		room := width - lipgloss.Width(g.name) - durationRoom - len("• ()")
		inner := truncate(unstyled(primaryArg(g.input)), room)
		return g.groupGlyph() + " " + g.name + "(" + inner + ")"
	}

	// Kind-based batch rendering
	kind := toolKind(g.name)
	glyph := toolKindGlyph(kind)
	if len(g.callNames) > 0 {
		return fmt.Sprintf("%s %d %ss · %s", glyph, g.count, kind, strings.Join(g.callNames[:min(len(g.callNames), 4)], " · "))
	}
	return fmt.Sprintf("%s %d %ss", glyph, g.count, kind)
}

// groupGlyph returns the icon for a group, honouring the same tone map a
// single call uses. The discarded and failed states keep their own markers
// because they are different outcomes, not just more of the same.
// For batches, it uses the kind glyph.
func (g toolGroup) groupGlyph() string {
	if g.discarded {
		return "⊘"
	}
	if g.failed {
		return "✗"
	}
	if g.count > 1 {
		return toolKindGlyph(toolKind(g.name))
	}
	return toolGlyph(g.name)
}

// inFlightLine is a group still running, with its elapsed time appended.
// It renders the same layout as groupLine but with a duration suffix so the
// reader can see how long the batch has been open. Re-rendering each frame
// keeps the clock moving while the tool is in flight.
func (g toolGroup) inFlightLine(width int) string {
	base := g.groupLine(width)
	if base == "" {
		return ""
	}
	elapsed := time.Since(g.start)
	return base + " · " + max(elapsed.Round(time.Millisecond), time.Millisecond).String()
}
