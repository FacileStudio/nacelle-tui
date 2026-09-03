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
type toolError struct {
	name     string
	err      string
	duration time.Duration
}

type toolGroup struct {
	name          string
	input         string
	count         int
	start         time.Time
	end           time.Time
	failed        bool
	discarded     bool
	callNames     []string
	callIDs       []string
	errors        []toolError
	tool          nacelle.ToolEvent
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

	kind := toolKind(g.name)
	glyph := g.groupGlyph()
	prefix := fmt.Sprintf("%s %d %ss", glyph, g.count, kind)
	if len(g.callNames) > 0 {
		args := strings.Join(g.callNames[:min(len(g.callNames), 4)], " · ")
		if width > 0 {
			room := width - lipgloss.Width(prefix) - len(" · ") - durationRoom
			if room > 0 {
				return prefix + " · " + truncate(unstyled(args), room)
			}
			return truncate(prefix, width-durationRoom)
		}
		return prefix + " · " + args
	}
	return prefix
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
	line := base + " · " + max(elapsed.Round(time.Millisecond), time.Millisecond).String()
	if width > 0 {
		line = truncate(line, width)
	}
	return line
}

func (g toolGroup) duration() time.Duration {
	if g.end.IsZero() {
		return time.Since(g.start)
	}
	return g.end.Sub(g.start)
}

func (g *toolGroup) finishCall(ev nacelle.ToolEvent) {
	g.tool = ev
	g.finishedCount++
	if ev.Err != nil {
		g.failed = true
		g.errors = append(g.errors, toolError{
			name:     ev.Name,
			err:      ev.Err.Error(),
			duration: ev.Duration,
		})
	}
	if ev.Discarded {
		g.discarded = true
	}
	if g.finishedCount >= g.count {
		g.end = time.Now()
	}
}
