package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle-tui/internal/layout"
	"github.com/FacileStudio/nacelle-tui/internal/status"
)

// parallelTasksView returns a view of the parallel subagent tasks, one line per
// task. A running task shows its title and an elapsed clock that the spinner
// tick redraws; a finished one shows its own spend from the result's usage map
// and the duration it took, so the per-subagent cost is visible rather than a
// single total copy-pasted onto every row.
func (m *Model) parallelTasksView() string {
	if len(m.parallelTasks) == 0 {
		return ""
	}
	var lines []string
	for _, tasks := range m.parallelTasks {
		for _, pt := range tasks {
			lines = append(lines, m.taskRow(pt))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) taskRow(pt parallelTaskInfo) string {
	const gap = 3
	clock := taskClock(pt)
	spend := taskSpend(pt.Usage)
	tail := strings.TrimSpace(clock + " " + spend)
	room := max(m.width-lipgloss.Width(tail)-gap, 0)
	left := taskTone(pt).Render("≫ " + layout.Truncate(taskTitle(pt), room))
	pad := max(m.width-lipgloss.Width(left)-lipgloss.Width(tail), 0)
	return left + strings.Repeat(" ", pad) + tail
}

func taskTone(pt parallelTaskInfo) lipgloss.Style {
	if !pt.Active {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
}

// taskClock is the elapsed time: live for a running task, frozen for a done one.
func taskClock(pt parallelTaskInfo) string {
	elapsed := time.Since(pt.Began)
	if !pt.Active && !pt.End.IsZero() {
		elapsed = pt.End.Sub(pt.Began)
	}
	return status.Lasted(elapsed)
}

// taskSpend formats one subagent's own token burn, empty until the result
// arrives with its per-task usage map.
func taskSpend(usage nacelle.Usage) string {
	var parts []string
	if usage.Cost > 0 {
		parts = append(parts, fmt.Sprintf("$%.4f", usage.Cost))
	}
	if usage.Total() > 0 {
		parts = append(parts,
			"↑"+status.ShortTokens(usage.InputTokens+usage.CacheCreationTokens),
			"↓"+status.ShortTokens(usage.OutputTokens))
	}
	return strings.Join(parts, " ")
}

// parallelTasksRows returns the number of parallel task rows for layout.
func (m *Model) parallelTasksRows() int {
	return parallelTaskRows(m.parallelTasks)
}
