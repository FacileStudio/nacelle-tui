// Package tasks manages the interactive task plan checklist.
package tasks

import (
	"sync/atomic"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FacileStudio/nacelle"
)

// NewTool returns a tool that lets the model manage a task list shown on screen.
func NewTool() nacelle.Tool {
	return tasksTool{}
}

// The five states a step can be in. They are the model's vocabulary, not
// this client's: they travel in the tool's JSON and are checked against these
// exact strings, so renaming one here renames it in every prompt the model
// has already been given.
const (
	statusTodo    = "pending"
	statusActive  = "in_progress"
	statusDone    = "completed"
	statusBlocked = "blocked"
	statusFailed  = "failed"
)

// TaskItem is one step of the plan the model laid out.
type TaskItem struct {
	Title  string `json:"title"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type taskItem = TaskItem

// TaskList is the whole plan, in the order the model wrote it.
type TaskList []TaskItem

type taskList = TaskList

// TaskUpdate is that same plan crossing a goroutine boundary.
type TaskUpdate TaskList

type taskUpdate = TaskUpdate

var reports = make(chan TaskUpdate, 16)

// ReportChan returns the channel carrying reported plans from the tool to the update loop.
func ReportChan() <-chan TaskUpdate {
	return reports
}

// currentPlan shares the latest plan across goroutines so the task tool
// can read it for incremental updates. Written by the update loop on every
// taskUpdate, read by the tool's goroutine on a step_update call.
//
// An atomic.Value is the right shape here because both sides are goroutines
// that never yield to each other's scheduler — the tool runs on the agent's
// goroutine while the model runs on bubbletea's. The value is nil until the
// first plan arrives, and callers check that.
var currentPlan atomic.Value

// Rows is how many lines view draws for this list. Layout reserves exactly
// this many, so the two must never be able to disagree — which is why it is
// this function and not a len() at both call sites. See queuedHeight, which
// carries the same warning about the same bug.
func (t TaskList) Rows() int {
	return len(t)
}

func (t TaskList) rows() int {
	return t.Rows()
}

// View is the plan as it stands, drawn between the blank row and the status line.
func (t TaskList) View(width int, muted lipgloss.Style) []string {
	lines := make([]string, 0, t.Rows())
	for _, item := range t {
		glyph := taskGlyphStyle(item.Status).Render(taskGlyph(item.Status))
		title := ansi.Truncate(ansi.Strip(item.Title), max(0, width-lipgloss.Width(glyph)-1), "…")
		if title == "" {
			lines = append(lines, glyph)
		} else {
			lines = append(lines, glyph+" "+muted.Render(title))
		}
	}
	return lines
}

func (t TaskList) view(width int, muted lipgloss.Style) []string {
	return t.View(width, muted)
}

// taskGlyph marks a step's state with a character that is text in every
// terminal, for the reason toolIcons gives: nothing with an emoji
// presentation, so the plan never depends on a colour font being installed.
// An unrecognised status draws as pending rather than as nothing, because a
// step with no marker at all reads as a wrapped title belonging to the line
// above.
func taskGlyph(status string) string {
	switch status {
	case statusDone:
		return "✓"
	case statusActive:
		return "●"
	case statusFailed:
		return "⊗"
	case statusBlocked:
		return "⊘"
	}
	return "○"
}

// taskGlyphStyle returns the colour for a step's marker, so a completed step
// shows green and a running step shows cyan, a failed step shows red and a
// blocked step shows yellow. Pending steps have no colour of their own —
// they fall through to the muted style the whole line already inherits.
func taskGlyphStyle(status string) lipgloss.Style {
	switch status {
	case statusDone:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	case statusActive:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	case statusFailed:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	case statusBlocked:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	}
	return lipgloss.NewStyle()
}
