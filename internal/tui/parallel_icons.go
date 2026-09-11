package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle-tui/internal/toolview"
)

// green, red and yellow are the three run-state colours a parallel task row's
// marker and tool glyph share with the transcript: green for succeeded, red for
// failed, and yellow while it is still running. The tool's own tone stands in
// for yellow on the glyph while its call is in flight.
var green = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
var red = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
var yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

// parallelMarker is the leading glyph of a task row: a yellow spinner while the
// task runs, a green check when it finished cleanly, a red cross when it failed.
func (m *Model) parallelMarker(pt parallelTaskInfo) string {
	if pt.Active {
		return yellow.Render(m.spin.View())
	}
	if pt.Err != "" {
		return red.Render("✗")
	}
	return green.Render("✓")
}

// parallelGlyph renders the task's currently-running tool icon with its name,
// coloured by the same run-state rules as the transcript: the tool's own tone
// while it runs, green the moment its call succeeds, red when it fails. Empty
// when the task is not running a tool.
func (m *Model) parallelGlyph(pt parallelTaskInfo) string {
	if pt.Tool == "" {
		return ""
	}
	style := toolview.ToolTone(pt.Tool)
	if pt.ToolOut == "ok" {
		style = green
	} else if pt.ToolOut != "" {
		style = red
	}
	return style.Render(toolview.ToolGlyph(pt.Tool) + " " + pt.Tool)
}
