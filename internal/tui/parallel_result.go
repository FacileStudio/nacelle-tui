package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FacileStudio/nacelle"
)

// parallelResult mirrors the JSON returned by parallel_subagent.
type parallelResult struct {
	Tasks  map[string]string        `json:"tasks,omitempty"`
	Errors map[string]string        `json:"errors,omitempty"`
	Usage  map[string]nacelle.Usage `json:"usage,omitempty"`
}

// handleParallelResult merges the fan-out result into the tracked tasks for
// this call. The call's rows are kept once done so each subagent's final spend
// stays visible under the prompt while the parent narrates; stranded() clears
// calls whose tasks have all finished at the next send or run end.
func (m *Model) handleParallelResult(rawResult string, toolID string, tasks []parallelTaskInfo) {
	var result parallelResult
	if err := json.Unmarshal([]byte(rawResult), &result); err != nil {
		m.parallelResultError(toolID, tasks, err)
		return
	}
	for i := range tasks {
		m.applyParallelResult(i, result, tasks)
	}
}

func (m *Model) applyParallelResult(i int, result parallelResult, tasks []parallelTaskInfo) {
	idx := strconv.Itoa(i)
	pt := &tasks[i]
	if errMsg, ok := result.Errors[idx]; ok {
		pt.Err = errMsg
	} else if res, ok := result.Tasks[idx]; ok {
		pt.Result = res
	}
	if u, ok := result.Usage[idx]; ok {
		pt.Usage = u
	}
	pt.End = time.Now()
	pt.Active = false
}

func (m *Model) parallelResultError(toolID string, tasks []parallelTaskInfo, err error) {
	for i := range tasks {
		tasks[i].Err = fmt.Sprintf("failed to parse result: %v", err)
		tasks[i].End = time.Now()
		tasks[i].Active = false
	}
	delete(m.parallelTasks, toolID)
}

// dropFinishedParallel forgets every parallel call whose tasks have all ended,
// so a completed fan-out's rows leave at the next send or run end rather than
// sitting under the prompt forever. Run from stranded(), which both send and
// settle reach.
func (m *Model) dropFinishedParallel() {
	for toolID, tasks := range m.parallelTasks {
		done := true
		for _, pt := range tasks {
			if pt.Active {
				done = false
				break
			}
		}
		if done {
			delete(m.parallelTasks, toolID)
		}
	}
}

// taskTitle collapses a task's prompt onto one line, so the running task row
// reads as an action instead of a pasted paragraph.
func taskTitle(pt parallelTaskInfo) string {
	return strings.Join(strings.Fields(pt.Task), " ")
}

// parallelTaskRows returns the row count for a map of parallel_subagent calls.
func parallelTaskRows(tasks map[string][]parallelTaskInfo) int {
	rows := 0
	for _, call := range tasks {
		rows += len(call)
	}
	return rows
}
