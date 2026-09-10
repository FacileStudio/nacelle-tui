package tui

import (
	"strings"

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
	if !m.validUpdate(u) {
		return watchUpdates()
	}
	pt := &m.parallelTasks[u.batch][u.idx]
	if u.spend {
		pt.Usage = pt.Usage.Add(u.usage)
		m.foldSubagentSpend(pt, u)
	} else {
		pt.Tool = u.tool
	}
	return watchUpdates()
}

func (m *Model) validUpdate(u subagentUpdate) bool {
	tasks, ok := m.parallelTasks[u.batch]
	return ok && u.idx >= 0 && u.idx < len(tasks) && tasks[u.idx].Active
}

// foldSubagentSpend joins a live spend to the session total for a detached
// fan-out, and remembers how much was folded so the task's final figure only
// adds the residual. A model-called fan-out stays out — see isDetachedBatch.
func (m *Model) foldSubagentSpend(pt *parallelTaskInfo, u subagentUpdate) {
	if !isDetachedBatch(u.batch) {
		return
	}
	m.spent = m.spent.Add(u.usage)
	pt.Ledgered = pt.Ledgered.Add(u.usage)
}

// isDetachedBatch reports whether a batch key belongs to a /parallel fan-out
// rather than to the model's parallel_subagent tool call. The two are told apart
// by the "detach" prefix launchDetached hands out, and they pay for it here: a
// model-callable fan-out's spend already reaches the session total through
// nacelle's Usage hook (delegations), so folding its live updates in again would
// double-count it. A detached fan-out has no Usage hook, so its live spend has
// to be folded here to reach the footer in real time.
func isDetachedBatch(batch string) bool {
	return strings.HasPrefix(batch, "detach")
}
