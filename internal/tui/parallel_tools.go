package tui

import (
	tea "charm.land/bubbletea/v2"
)

// This file owns the parallel fan-out's live tool-call path: how a nested
// task's running tool travels from the delegate's stream to its row. It is
// separate from parallel_detached.go (the launcher and result handlers) to
// stay under filet's per-file function cap, the same reason parallel_titles.go
// owns the title path.

// subagentTool carries one nested task's live tool call back to the update
// loop: which fan-out, which task, and the tool it is running right now.
type subagentTool struct {
	batch string
	idx   int
	tool  string
}

// subagentTools is the channel the delegated fan-outs' live tool calls arrive
// on, drained by watchTools on the update loop — the one thread that may touch
// the task rows.
var subagentTools = make(chan subagentTool, 64)

func watchTools() tea.Cmd {
	return func() tea.Msg {
		return <-subagentTools
	}
}

// ReportSubagentTool is mounted as the parallel tool's Tool hook: nacelle
// calls it on each nested task's stream as a tool begins, and it hands the
// event to the update loop. batch is the fan-out key the SDK forwards, so a
// host with several overlapping fan-outs routes each call to the right batch.
func ReportSubagentTool(batch string, idx int, tool string) {
	subagentTools <- subagentTool{batch: batch, idx: idx, tool: tool}
}

// recordTool applies a nested task's live tool call to its row and re-arms the
// watch. It only touches still-running tasks, so a tool event that trails a
// finished result cannot resurrect a row.
func (m *Model) recordTool(t subagentTool) tea.Cmd {
	if tasks, ok := m.parallelTasks[t.batch]; ok && t.idx >= 0 && t.idx < len(tasks) {
		if tasks[t.idx].Active {
			tasks[t.idx].Tool = t.tool
		}
	}
	return watchTools()
}
