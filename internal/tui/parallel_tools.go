package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// This file owns the parallel fan-out's live update path: how a nested task's
// running tool AND its streaming spend travel from the delegate to its row. The
// two share one channel because neither is worth its own arm of the update-loop
// dispatcher (route is at filet's floor), and both are folded the same way. It
// is separate from parallel_detached.go (the launcher and result handlers) to
// stay under filet's per-file function cap, the same reason parallel_titles.go
// owns the title path.

// subagentUpdate is one nested task's live progress: which fan-out, which task,
// and either the tool it is running now (spend false) or an incremental turn
// spend (spend true). Both fold into the same row without growing the switch.
type subagentUpdate struct {
	batch string
	idx   int
	tool  string
	usage nacelle.Usage
	spend bool
}

// subagentUpdates is the channel the delegated fan-outs' live updates arrive on,
// drained by watchUpdates on the update loop — the one thread that may touch the
// task rows.
var subagentUpdates = make(chan subagentUpdate, 128)

func watchUpdates() tea.Cmd {
	return func() tea.Msg {
		return <-subagentUpdates
	}
}

// ReportSubagentTool is mounted as the parallel tool's Tool hook: nacelle
// calls it on each nested task's stream as a tool begins, and it hands the
// event to the update loop. batch is the fan-out key the SDK forwards, so a
// host with several overlapping fan-outs routes each call to the right batch.
func ReportSubagentTool(batch string, idx int, tool string) {
	subagentUpdates <- subagentUpdate{batch: batch, idx: idx, tool: tool}
}

// ReportSubagentUsage is mounted as the parallel tool's LiveUsage hook: nacelle
// calls it on each nested task's turn as its spend streams, and it hands the
// spend to the update loop so a still-running task's counter moves before the
// result arrives. batch is the fan-out key the SDK forwards, matching Results'.
func ReportSubagentUsage(batch string, idx int, usage nacelle.Usage) {
	subagentUpdates <- subagentUpdate{batch: batch, idx: idx, usage: usage, spend: true}
}

// recordUpdate applies one live update to its task's row and re-arms the watch:
// a tool call to the running-tool column, a spend to the running counter. It
// only touches still-running tasks, so an update that trails a finished result
// can neither resurrect a row nor double-count it.
func (m *Model) recordUpdate(u subagentUpdate) tea.Cmd {
	tasks, ok := m.parallelTasks[u.batch]
	if ok && u.idx >= 0 && u.idx < len(tasks) {
		if tasks[u.idx].Active {
			if u.spend {
				tasks[u.idx].Usage = tasks[u.idx].Usage.Add(u.usage)
			} else {
				tasks[u.idx].Tool = u.tool
			}
		}
	}
	return watchUpdates()
}
