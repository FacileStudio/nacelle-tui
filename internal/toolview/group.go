package toolview

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FacileStudio/nacelle"
)

// ToolError captures a single tool failure within a call or batch.
type ToolError struct {
	Name     string
	Err      string
	Duration time.Duration
}

// Group represents a single tool call or a batch of same-kind calls.
type Group struct {
	Name          string
	Input         string
	Count         int
	Start         time.Time
	End           time.Time
	Failed        bool
	Discarded     bool
	CallNames     []string
	CallIDs       []string
	Errors        []ToolError
	Tool          nacelle.ToolEvent
	FinishedCount int
}

// GroupLine renders the summary line for a single call or batch.
func (g Group) GroupLine(width int) string {
	if g.Count <= 1 {
		return ToolLineSource(g.Name, g.Input, width, g.Tool.Source)
	}

	kind := ToolKind(g.Name)
	glyph := g.GroupGlyph()
	prefix := fmt.Sprintf("%s %d %ss", glyph, g.Count, kind)
	if len(g.CallNames) > 0 {
		args := strings.Join(g.CallNames[:min(len(g.CallNames), 4)], " · ")
		if width > 0 {
			room := width - lipgloss.Width(prefix) - len(" · ") - DurationRoom
			if room > 0 {
				return prefix + " · " + truncate(ansi.Strip(args), room)
			}
			return truncate(prefix, width-DurationRoom)
		}
		return prefix + " · " + args
	}
	return prefix
}

// GroupGlyph returns the representative marker icon for the group.
func (g Group) GroupGlyph() string {
	if g.Discarded {
		return "⊘"
	}
	if g.Failed {
		return "✗"
	}
	if g.Count > 1 {
		return ToolKindGlyph(ToolKind(g.Name))
	}
	return ToolSourceGlyph(g.Name, g.Tool.Source)
}

// InFlightLine renders a running group with its elapsed duration.
func (g Group) InFlightLine(width int) string {
	base := g.GroupLine(width)
	if base == "" {
		return ""
	}
	elapsed := time.Since(g.Start)
	line := base + " · " + max(elapsed.Round(time.Millisecond), time.Millisecond).String()
	if width > 0 {
		line = truncate(line, width)
	}
	return line
}

// Duration returns the elapsed or total runtime for the group.
func (g Group) Duration() time.Duration {
	if g.End.IsZero() {
		return time.Since(g.Start)
	}
	return g.End.Sub(g.Start)
}

// FinishCall records a completed tool event in the group.
func (g *Group) FinishCall(ev nacelle.ToolEvent) {
	g.Tool = ev
	g.FinishedCount++
	if ev.Err != nil {
		g.Failed = true
		g.Errors = append(g.Errors, ToolError{
			Name:     ev.Name,
			Err:      ev.Err.Error(),
			Duration: ev.Duration,
		})
	}
	if ev.Discarded {
		g.Discarded = true
	}
	if g.FinishedCount >= g.Count {
		g.End = time.Now()
	}
}
