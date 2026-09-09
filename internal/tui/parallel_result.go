package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// parallelResult mirrors the JSON returned by parallel_subagent.
type parallelResult struct {
	Tasks  map[string]string `json:"tasks,omitempty"`
	Errors map[string]string `json:"errors,omitempty"`
}

// maxParallelResultLines caps the rendered text of a single subagent result so
// one long answer cannot push the prompt off the bottom of the terminal.
const maxParallelResultLines = 6

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

// truncateLines returns at most n lines from s. A trailing newline is stripped
// so a partial last line is not dropped on the floor.
func truncateLines(s string, n int) []string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// taskStatusLines returns the status and optional result lines for one parallel task.
func taskStatusLines(pt parallelTaskInfo) []string {
	switch {
	case pt.Active:
		return []string{"  · running..."}
	case pt.Err != "":
		return []string{"  · error: " + pt.Err}
	case pt.Result != "":
		out := make([]string, 0, len(truncateLines(pt.Result, maxParallelResultLines)))
		for _, line := range truncateLines(pt.Result, maxParallelResultLines) {
			out = append(out, "  · "+line)
		}
		return out
	default:
		return nil
	}
}

// callLines returns all rendered lines for one parallel_subagent call's tasks.
func callLines(tasks []parallelTaskInfo) []string {
	var lines []string
	for _, pt := range tasks {
		lines = append(lines, "≫ "+pt.Task)
		lines = append(lines, taskStatusLines(pt)...)
	}
	return lines
}

// parallelTaskRows returns the row count for a map of parallel_subagent calls.
func parallelTaskRows(tasks map[string][]parallelTaskInfo) int {
	rows := 0
	for _, call := range tasks {
		for _, pt := range call {
			rows++
			if pt.Active || pt.Err != "" || pt.Result != "" {
				rows++
			}
			if pt.Result != "" {
				rows += len(truncateLines(pt.Result, maxParallelResultLines)) - 1
			}
		}
	}
	return rows
}
