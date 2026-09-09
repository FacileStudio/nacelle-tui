package tui

import (
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

// The parallel list under the input prompt shows one compact line per running
// subagent: a ≫ glyph and the task's first line collapsed, with the run's
// combined spend pushed to the right margin. A pasted multi-line task prompt
// must not spill across rows.
func TestParallelTasksViewRendersOneCompactLinePerTask(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["t0"] = make([]parallelTaskInfo, 2)
	m.parallelTasks["t0"][0] = parallelTaskInfo{Task: "analyze http://x.dev\nfor TODO comments", Active: true}
	m.parallelTasks["t0"][1] = parallelTaskInfo{Task: "summarize the findings", Active: true}
	m.run.usage = nacelle.Usage{InputTokens: 128, OutputTokens: 42, Cost: 0.01}

	raw := m.parallelTasksView()
	got := visible(raw)

	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("view = %q, want one row per task", got)
	}
	for _, line := range lines {
		if !strings.Contains(line, "≫") {
			t.Errorf("line %q missing the ≫ glyph", line)
		}
		if !strings.Contains(line, "↑128") || !strings.Contains(line, "↓42") || !strings.Contains(line, "$0.0100") {
			t.Errorf("line %q missing the spend", line)
		}
	}
	if !strings.Contains(raw, "\x1b[33m≫") {
		t.Errorf("glyph not styled yellow: %q", raw)
	}
}

// parallelTaskRows is what layout reserves beneath the prompt, so it has to
// match the one-row-per-task shape the view draws.
func TestParallelTaskRowsCountsTasks(t *testing.T) {
	if got := parallelTaskRows(nil); got != 0 {
		t.Errorf("parallelTaskRows(nil) = %d, want 0", got)
	}

	tasks := make(map[string][]parallelTaskInfo)
	tasks["a"] = make([]parallelTaskInfo, 2)
	tasks["b"] = make([]parallelTaskInfo, 1)
	if got := parallelTaskRows(tasks); got != 3 {
		t.Errorf("parallelTaskRows = %d, want 3", got)
	}
}
