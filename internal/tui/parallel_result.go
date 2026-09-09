package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle-tui/internal/layout"
)

// parallelResult mirrors the JSON returned by parallel_subagent.
type parallelResult struct {
	Tasks  map[string]string `json:"tasks,omitempty"`
	Errors map[string]string `json:"errors,omitempty"`
}

// handleParallelResult merges the fan-out result into the tracked tasks for
// this call, then removes the call's entry so it does not sit in memory.
func (m *Model) handleParallelResult(rawResult string, toolID string, tasks []parallelTaskInfo) {
	var result parallelResult
	if err := json.Unmarshal([]byte(rawResult), &result); err != nil {
		m.parallelResultError(toolID, tasks, err)
		return
	}
	for i := range tasks {
		m.applyParallelResult(i, result, tasks)
	}
	delete(m.parallelTasks, toolID)
}

func (m *Model) applyParallelResult(i int, result parallelResult, tasks []parallelTaskInfo) {
	idx := strconv.Itoa(i)
	pt := &tasks[i]
	if errMsg, ok := result.Errors[idx]; ok {
		pt.Err = errMsg
	} else if res, ok := result.Tasks[idx]; ok {
		pt.Result = res
	}
	pt.Active = false
}

func (m *Model) parallelResultError(toolID string, tasks []parallelTaskInfo, err error) {
	for i := range tasks {
		tasks[i].Err = fmt.Sprintf("failed to parse result: %v", err)
		tasks[i].Active = false
	}
	delete(m.parallelTasks, toolID)
}

// taskTitle collapses a task's prompt onto one line, so the running task row
// reads as an action instead of a pasted paragraph.
func taskTitle(pt parallelTaskInfo) string {
	return strings.Join(strings.Fields(pt.Task), " ")
}

// callLines returns one rendered line per parallel task: the task's shortened
// title in yellow on the left, and the run's combined spend against the right
// margin. Results and errors arrive all at once and drop the call from the map
// (see handleParallelResult), so this only ever shows tasks still running.
func callLines(tasks []parallelTaskInfo, width int, spend string) []string {
	const gap = 3
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	var lines []string
	for _, pt := range tasks {
		sWidth := lipgloss.Width(spend)
		room := width - sWidth - gap
		title := layout.Truncate(taskTitle(pt), room)
		left := yellow.Render("≫ " + title)
		pad := max(width-sWidth-lipgloss.Width(left), 0)
		lines = append(lines, left+strings.Repeat(" ", pad)+spend)
	}
	return lines
}

// parallelTaskRows returns the row count for a map of parallel_subagent calls.
func parallelTaskRows(tasks map[string][]parallelTaskInfo) int {
	rows := 0
	for _, call := range tasks {
		rows += len(call)
	}
	return rows
}
