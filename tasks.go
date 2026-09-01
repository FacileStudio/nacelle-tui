package main

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// The three states a step can be in. They are the model's vocabulary, not
// this client's: they travel in the tool's JSON and are checked against these
// exact strings, so renaming one here renames it in every prompt the model
// has already been given.
const (
	statusTodo   = "pending"
	statusActive = "in_progress"
	statusDone   = "completed"
)

// taskItem is one step of the plan the model laid out.
type taskItem struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

// taskList is the whole plan, in the order the model wrote it.
type taskList []taskItem

// taskUpdate is that same plan crossing a goroutine boundary, as a message
// the update loop routes. It is a distinct type so the switch in route can
// tell "the model reported a plan" from "here is a plan", which are the same
// bytes and very different events.
type taskUpdate taskList

// reports carries a reported plan from the tool's goroutine to the update
// loop, the way delegations carries a delegated spend.
//
// Buffered, and the send blocks when it is full rather than dropping. A
// dropped plan is not a frame of animation nobody misses: the list is a
// snapshot that replaces the last one, so losing one leaves the screen
// showing a step as still running that finished minutes ago, and nothing
// later ever corrects it. A blocked tool call is one turn arriving late.
var reports = make(chan taskUpdate, 16)

// taskRows is how many steps are drawn before the rest are counted in one
// line instead, for the reason queuedRows spells out: layout reserves a row
// per line drawn here, so an unbounded list is a transcript squeezed to
// nothing and a prompt pushed off the bottom. A plan is a reminder of where
// the work is, not a document.
const taskRows = 5

// rows is how many lines view draws for this list. layout reserves exactly
// this many, so the two must never be able to disagree — which is why it is
// this function and not a len() at both call sites. See queuedHeight, which
// carries the same warning about the same bug.
func (t taskList) rows() int {
	if len(t) <= taskRows {
		return len(t)
	}
	return taskRows + 1
}

// window is the slice of steps that fits on screen, and how many are left
// out.
//
// It scrolls to the step in flight rather than always showing the top of the
// list. A twenty-step plan whose first five are done would otherwise draw
// five ticks and a count, which is the one part of the plan nobody needs to
// see: the reader is watching for what is happening now. One finished step is
// kept above it for context, because a running step with nothing before it
// reads as the beginning of the plan.
func (t taskList) window() (taskList, int) {
	if len(t) <= taskRows {
		return t, 0
	}
	start := 0
	for i, item := range t {
		if item.Status == statusActive {
			start = min(max(i-1, 0), len(t)-taskRows)
			break
		}
	}
	return t[start : start+taskRows], len(t) - taskRows
}

// view is the plan as it stands, drawn between the blank row and the status
// line.
//
// Each line is truncated to the width rather than allowed to wrap, because
// layout reserved exactly one row for it. A wrapped title would push the
// prompt off the bottom of the screen, which is the same fault the dropdown
// and the queue each had once.
//
// Titles are stripped of escape sequences before they are measured. They are
// written by the model, so a title ending in a bare "\x1b[7m" costs no cells,
// survives the width check, and leaves the terminal in reverse video for
// everything printed after it — see unstyled.
func (t taskList) view(width int, muted lipgloss.Style) []string {
	shown, hidden := t.window()
	lines := make([]string, 0, t.rows())
	for _, item := range shown {
		glyph := taskGlyphStyle(item.Status).Render(taskGlyph(item.Status))
		title := truncate(unstyled(item.Title), max(0, width-lipgloss.Width(glyph)-1))
		if title == "" {
			lines = append(lines, glyph)
		} else {
			lines = append(lines, glyph+" "+muted.Render(title))
		}
	}
	if hidden > 0 {
		lines = append(lines, muted.Render(truncate(fmt.Sprintf("… and %d more", hidden), width)))
	}
	return lines
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
		return "▶"
	}
	return "○"
}

// taskGlyphStyle returns the colour for a step's marker, so a completed step
// shows green and a running step shows cyan. Pending steps have no colour of
// their own — they fall through to the muted style the whole line already
// inherits.
func taskGlyphStyle(status string) lipgloss.Style {
	switch status {
	case statusDone:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	case statusActive:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	}
	return lipgloss.NewStyle()
}

// watchTasks waits for one reported plan and hands it to the update loop as a
// message. Every handling re-arms it, so it listens for the whole session the
// same way watchDelegations does.
//
// Init batches this alongside watchDelegations, and has to: recordTasks is the
// only other thing that arms it, so a watcher armed nowhere else would not be
// listening until the first plan had already been recorded. The buffer means
// those first reports are not lost, but they are not drawn either, and the tool
// blocks once sixteen have piled up behind a listener that never started.
func watchTasks() tea.Cmd {
	return func() tea.Msg {
		return <-reports
	}
}

// recordTasks stores the plan the model last reported and re-arms the watcher.
//
// It is a method rather than four lines in route for the reason
// recordDelegation is: route is a switch that says where a message goes, and
// filet caps it at thirty lines — a case that does its own work there is a case
// that pushes the next one over.
func (m *model) recordTasks(reported taskUpdate) tea.Cmd {
	m.tasks = taskList(reported)
	m.layout(m.windowHeight)
	return watchTasks()
}
