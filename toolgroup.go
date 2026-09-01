package main

import (
	"fmt"
	"strings"
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

// groupElapsed is how long this group has been running, or how long it ran
// for if it has finished.
func (g toolGroup) groupElapsed(now time.Time) time.Duration {
	if g.end.IsZero() {
		return now.Sub(g.start)
	}
	return g.end.Sub(g.start)
}

// sameTool reports whether a second event should join an existing group. It
// is deliberately narrow: the name has to match, the input has to match, and
// the first event has to be a call awaiting a result. A result never starts a
// group, because two results in a row are two different calls finishing, and
// merging them would paint one row for two outputs.
//
// unused — run.go's beginTool does the same check inline and is the one
// consumer — but kept for the reference it provides to the grouping contract.
// func sameTool(g *toolGroup, ev nacelle.ToolEvent) bool {
// 	if g == nil || g.count == 0 {
// 		return false
// 	}
// 	return g.end.IsZero() &&
// 		ev.Name == g.tool.Name &&
// 		ev.Input == g.tool.Input
// }

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

// groupStatus is the suffix a group's line carries: the outcome, or the
// elapsed time if it is still running. It is the same shape the single-call
// line wears in toolline.go, and it is kept separate so the caller can decide
// whether to show it — stranded and finished both need the core line, but
// stranded must not say how long a call that never completed took.
func (g toolGroup) groupStatus(now time.Time) string {
	switch {
	case g.discarded:
		return ""
	case g.failed:
		return " · failed after " + took(g.groupElapsed(now))
	case g.end.IsZero():
		return " · running " + formatDuration(g.groupElapsed(now))
	default:
		return " · " + took(g.groupElapsed(now))
	}
}

// groupPainted is groupLine in the colour a finished group's line carries,
// so the live region can render it the same way a printed line is painted.
func (g toolGroup) groupPainted(now time.Time, width int) string {
	return toolLinePainted(g.groupLine(width) + g.groupStatus(now))
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

// groupReport is the one-line summary under the icon. When the group has
// finished it reports the outcome; when it is still open it reports that it
// is still running, with the elapsed time so a long chain of identical calls
// is distinguishable from a single slow one.
func (g toolGroup) groupReport(now time.Time) string {
	if g.count == 0 {
		return ""
	}
	if g.discarded {
		return "declined"
	}
	if g.failed {
		return "failed"
	}
	if g.end.IsZero() {
		return "running " + formatDuration(g.groupElapsed(now))
	}
	return formatDuration(g.groupElapsed(now))
}

// formatDuration renders a duration the way the rest of the client does, in
// whole seconds with a trailing "s". It lives here rather than in thinking.go
// because the thinking path formats its own clock differently (it shows
// minutes for long reasoning), and the two should not share a helper that
// quietly makes them agree.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	return strings.TrimRight(strings.TrimRight(
		strings.ReplaceAll(d.Round(time.Second).String(), "m", "m "), "0"), " ") + "s"
}
