package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

// /parallel splits a comma-separated list of tasks and sends them to the
// model as a prompt asking it to use the parallel_subagent tool. Each task
// is trimmed and empty entries are dropped.
func TestParallelCommandSplitsTasksAndStartsARun(t *testing.T) {
	m := sized()
	m.agent = answering(t)

	m.prompt.SetValue("/parallel analyze this codebase, search for TODO comments, summarize the findings")
	m.ask()

	if !m.run.busy {
		t.Fatal("/parallel did not start a run")
	}
	if len(m.conversation) != 1 {
		t.Fatalf("conversation = %v, want the prompt sent", m.conversation)
	}
	sent := m.conversation[0].Parts[0].(nacelle.Text).Text
	if !strings.Contains(sent, "parallel_subagent") {
		t.Errorf("sent = %q, want the tool named", sent)
	}
	for _, task := range []string{"analyze this codebase", "search for TODO comments", "summarize the findings"} {
		if !strings.Contains(sent, task) {
			t.Errorf("sent = %q, want task %q", sent, task)
		}
	}
}

// A single task still routes through the parallel path; the difference from
// the single subagent is the concurrency cap, not the call shape.
func TestParallelCommandWithSingleTaskStillStartsARun(t *testing.T) {
	m := sized()
	m.agent = answering(t)

	m.prompt.SetValue("/parallel just one task")
	m.ask()

	if !m.run.busy {
		t.Fatal("/parallel with one task did not start a run")
	}
	if len(m.conversation) != 1 {
		t.Fatalf("conversation = %v, want the prompt sent", m.conversation)
	}
	sent := m.conversation[0].Parts[0].(nacelle.Text).Text
	if !strings.Contains(sent, "just one task") {
		t.Errorf("sent = %q, want the task", sent)
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
