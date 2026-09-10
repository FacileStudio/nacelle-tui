package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// This file is the detached /parallel fan-out: the launcher that runs nested
// agents off the main run, and the result handlers that fold their outcomes
// back into the update loop. It is separate from parallel_cmd.go (the parser)
// to stay under filter's per-file function cap; parallel_result.go owns the
// model-callable tool's result path.

// detachedResult is one subagent's outcome, or a launch failure, on its way
// back to the update loop from the fan-out goroutine. idx is the task's index
// in the batch; a negative idx means the whole batch failed to start and marks
// every still-running task.
type detachedResult struct {
	batch  string
	idx    int
	result string
	err    string
	usage  nacelle.Usage
}

// detached is the channel the fan-out goroutine posts results to, drained by
// watchDetached on the update loop — the one thread that may touch the task
// rows and the session total.
var detached = make(chan detachedResult, 64)

func watchDetached() tea.Cmd {
	return func() tea.Msg {
		return <-detached
	}
}

// launchDetached starts one fan-out without touching the main run. It creates
// the batch's row state, says in the main thread that the agents were started,
// and hands the actual delegation to a background goroutine; busy stays false,
// so the prompt stays live.
func (m *Model) launchDetached(tasks []string) tea.Cmd {
	id := m.nextDetachID()
	if m.parallelTasks == nil {
		m.parallelTasks = make(map[string][]parallelTaskInfo)
	}
	list := make([]parallelTaskInfo, len(tasks))
	for i, t := range tasks {
		list[i] = parallelTaskInfo{Task: t, Began: time.Now(), Active: true}
	}
	m.parallelTasks[id] = list
	m.titleParallelTasks(id, tasks)
	m.say(fromClient, fmt.Sprintf("started %d parallel agents", len(tasks)))
	m.layout(m.windowHeight)

	cfg := m.delegate
	go func() {
		results, err := nacelle.DelegateParallel(context.Background(), cfg, tasks, nacelle.ParallelSubAgentOptions{
			Approve: delegateApprove(cfg),
		})
		if err != nil {
			detached <- detachedResult{batch: id, idx: -1, err: err.Error()}
			return
		}
		for {
			next, open := <-results
			if !open {
				break
			}
			detached <- detachedResult{batch: id, idx: next.Index, result: next.Result, err: next.Err, usage: next.Usage}
		}
	}()
	return nil
}

// nextDetachID hands out a batch key for a detached fan-out. The "detach"
// prefix keeps it apart from the model-tool path's nacelle tool IDs, which are
// the other kind of key the parallelTasks map holds.
func (m *Model) nextDetachID() string {
	m.detachedSeq++
	return "detach" + strconv.Itoa(m.detachedSeq)
}

// delegateApprove is the approval policy the detached subagents answer to. It
// is the same rule agent.delegateApprovals applies to the model-callable tool:
// a delegate inherits the parent's policy, and a session with approvals off
// hands the delegate an allow-all rather than the SDK's deny-all default.
// The rule lives here rather than in agent because agent imports tui, so tui
// cannot import it back — this is the mirror that keeps the two linked.
func delegateApprove(cfg nacelle.Config) nacelle.Approve {
	if cfg.Approve != nil {
		return cfg.Approve
	}
	return func(context.Context, string, json.RawMessage) bool { return true }
}

// recordDetached applies one detached subagent result and re-arms the watch.
// A task's own spend joins the session total so the footer does not lie about
// work that ran outside any parent run.
func (m *Model) recordDetached(r detachedResult) tea.Cmd {
	if tasks, ok := m.parallelTasks[r.batch]; ok {
		m.applyDetached(tasks, r)
		m.layout(m.windowHeight)
	}
	return watchDetached()
}

// applyDetached routes one result to its task, or — negative index, the
// launch-failed marker — fails every still-running task in the batch.
func (m *Model) applyDetached(tasks []parallelTaskInfo, r detachedResult) {
	if r.idx >= 0 && r.idx < len(tasks) {
		m.finishDetached(&tasks[r.idx], r)
		return
	}
	for i := range tasks {
		m.failDetached(tasks, i, r.err)
	}
}

// finishDetached records a task's outcome and spend, and marks it done.
func (m *Model) finishDetached(pt *parallelTaskInfo, r detachedResult) {
	if r.err != "" {
		pt.Err = r.err
	} else {
		pt.Result = r.result
	}
	pt.Usage = r.usage
	pt.End = time.Now()
	pt.Active = false
	if r.usage.Total() > 0 {
		m.spent = m.spent.Add(r.usage)
	}
}

// failDetached marks one task failed; it only touches running tasks, so a
// launch error after some results arrived never clobbers what already landed.
func (m *Model) failDetached(tasks []parallelTaskInfo, i int, err string) {
	if !tasks[i].Active {
		return
	}
	tasks[i].Err = err
	tasks[i].End = time.Now()
	tasks[i].Active = false
}
