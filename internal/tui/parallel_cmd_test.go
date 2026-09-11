package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

// /parallel no longer routes through the parent model. A typed fan-out is
// detached: no run starts, no conversation line appears, and the main thread
// stays free so the reader can keep chatting while the agents grind. The
// command announces the fan-out in the main thread rather than queueing behind
// a busy run.
func TestParallelCommandLaunchesDetachedFanOut(t *testing.T) {
	m := sized()
	m.delegate = nacelle.Config{Backend: silent{}, System: "s", MaxIterations: 1}

	m.prompt.SetValue("/parallel analyze this codebase, search for TODO comments, summarize the findings")
	printed := printedBy(m.ask())

	if m.run.busy {
		t.Error("/parallel started a run; a detached fan-out must leave the main thread free")
	}
	if len(m.conversation) != 0 {
		t.Errorf("conversation = %v, want nothing sent to the parent", m.conversation)
	}
	if !strings.Contains(printed, "✓ parallel agents started") {
		t.Errorf("printed = %q, want the green started announcement", printed)
	}
}

// The detached fan-out registers its tasks as one active row batch under the
// prompt, a row per task, so the reader sees exactly what is grinding.
func TestParallelCommandCreatesTaskRows(t *testing.T) {
	m := sized()
	m.delegate = nacelle.Config{Backend: silent{}, System: "s", MaxIterations: 1}

	m.prompt.SetValue("/parallel analyze this, summarize that")
	m.ask()

	if len(m.parallelTasks) != 1 {
		t.Errorf("parallelTasks = %v, want one batch", m.parallelTasks)
	}
	for _, batch := range m.parallelTasks {
		if len(batch) != 2 {
			t.Errorf("batch rows = %d, want 2", len(batch))
		}
	}
}

// A single task still routes through the detached parallel machinery; the
// delegate takes a task list, so one item is the smallest fan-out, and the
// fan-out still must not take the main run.
func TestParallelCommandWithSingleTaskIsStillDetached(t *testing.T) {
	m := sized()
	m.delegate = nacelle.Config{Backend: silent{}, System: "s", MaxIterations: 1}

	m.prompt.SetValue("/parallel just one task")
	printed := printedBy(m.ask())

	if m.run.busy {
		t.Error("/parallel with one task started a run")
	}
	if len(m.parallelTasks) != 1 {
		t.Errorf("parallelTasks = %v, want one batch", m.parallelTasks)
	}
	if !strings.Contains(printed, "✓ parallel agent started") {
		t.Errorf("printed = %q, want the green started announcement", printed)
	}
}

// Empty tasks after splitting should not start a run.
func TestParallelCommandWithEmptyTasksDoesNotStartARun(t *testing.T) {
	m := sized()

	m.prompt.SetValue("/parallel , , ")
	printed := printedBy(m.ask())

	if m.run.busy {
		t.Error("/parallel with empty tasks started a run")
	}
	if !strings.Contains(printed, "usage: /parallel") {
		t.Errorf("printed = %q, want usage message", printed)
	}
}

// /parallel with no arguments at all should not start a run.
func TestParallelCommandWithNoArgumentsDoesNotStartARun(t *testing.T) {
	m := sized()

	m.prompt.SetValue("/parallel")
	printed := printedBy(m.ask())

	if m.run.busy {
		t.Error("/parallel with no arguments started a run")
	}
	if !strings.Contains(printed, "usage: /parallel") {
		t.Errorf("printed = %q, want usage message", printed)
	}
}

// splitParallelTasks trims each entry and drops empties, so whitespace and
// trailing commas do not produce phantom tasks.
func TestSplitParallelTasks(t *testing.T) {
	got := splitParallelTasks("a, b , c")
	want := []string{"a", "b", "c"}
	if !slices.Equal(got, want) {
		t.Errorf("splitParallelTasks = %v, want %v", got, want)
	}

	got = splitParallelTasks("only one")
	if len(got) != 1 || got[0] != "only one" {
		t.Errorf("splitParallelTasks = %v, want [only one]", got)
	}

	got = splitParallelTasks(",,")
	if len(got) != 0 {
		t.Errorf("splitParallelTasks = %v, want empty", got)
	}
}

// recordDetached applies a subagent's outcome to its row and folds the task's
// own spend into the session total, while a still-running sibling stays
// untouched — each row completes on its own schedule.
func TestRecordDetachedAppliesOneResult(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["d0"] = make([]parallelTaskInfo, 2)
	m.parallelTasks["d0"][0] = parallelTaskInfo{Task: "one", Active: true}
	m.parallelTasks["d0"][1] = parallelTaskInfo{Task: "two", Active: true}
	before := m.spent

	m.recordDetached(detachedResult{batch: "d0", idx: 0, result: "r0", usage: nacelle.Usage{OutputTokens: 100, Cost: 0.01}})

	t0 := &m.parallelTasks["d0"][0]
	if t0.Result != "r0" || t0.Active {
		t.Errorf("task 0 not recorded: result=%q active=%v", t0.Result, t0.Active)
	}
	if m.parallelTasks["d0"][1].Active == false {
		t.Error("task 1 was marked done alongside its sibling")
	}
	if m.spent.OutputTokens != before.OutputTokens+100 {
		t.Errorf("session spend not credited with the task's usage")
	}
}

// A launch failure — negative index — marks every still-active task in the
// batch failed, so a fan-out that cannot start does not leave rows spinning.
func TestRecordDetachedLaunchErrorFailsTheBatch(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["d0"] = make([]parallelTaskInfo, 2)
	m.parallelTasks["d0"][0] = parallelTaskInfo{Task: "one", Active: true}
	m.parallelTasks["d0"][1] = parallelTaskInfo{Task: "two", Active: true}

	m.recordDetached(detachedResult{batch: "d0", idx: -1, err: "boom"})

	for _, pt := range m.parallelTasks["d0"] {
		if pt.Active || pt.Err != "boom" {
			t.Errorf("task not failed: active=%v err=%q", pt.Active, pt.Err)
		}
	}
}

// recordDetached ignores a result for a batch it does not know, so a late
// result from a cleared fan-out cannot resurrect rows.
func TestRecordDetachedIgnoresUnknownBatch(t *testing.T) {
	m := sized()
	m.recordDetached(detachedResult{batch: "ghost", idx: 0, result: "r", usage: nacelle.Usage{}})
	if len(m.parallelTasks) != 0 {
		t.Errorf("unknown batch created state: %v", m.parallelTasks)
	}
}
