package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

// The parallel list under the input prompt shows one compact line per running
// subagent: a ≫ glyph, the task's first line collapsed, and a live elapsed
// clock. A pasted multi-line task prompt must not spill across rows.
func TestParallelTasksViewRendersOneCompactLinePerTask(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["t0"] = make([]parallelTaskInfo, 2)
	began := time.Now()
	m.parallelTasks["t0"][0] = parallelTaskInfo{Task: "analyze http://x.dev\nfor TODO comments", Began: began, Active: true}
	m.parallelTasks["t0"][1] = parallelTaskInfo{Task: "summarize the findings", Began: began, Active: true}

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
	}
	if !strings.Contains(raw, "\x1b[33m≫") {
		t.Errorf("glyph not styled yellow: %q", raw)
	}
}

// A finished task reports its own spend from the result's per-task usage map
// instead of a single global total copied onto every row — the per-subagent
// cost the old view hid.
func TestParallelTasksViewShowsPerTaskSpendOnCompletion(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["t0"] = make([]parallelTaskInfo, 1)
	pt := &m.parallelTasks["t0"][0]
	*pt = parallelTaskInfo{
		Task:   "scrape the pricing page",
		Began:  time.Now(),
		End:    time.Now(),
		Usage:  nacelle.Usage{InputTokens: 2048, OutputTokens: 300, Cost: 0.02},
		Active: false,
	}

	got := visible(m.parallelTasksView())

	if !strings.Contains(got, "↑2.0k") || !strings.Contains(got, "↓300") || !strings.Contains(got, "$0.0200") {
		t.Errorf("completed row missing the per-task spend, got %q", got)
	}
	if !strings.Contains(got, "s") {
		t.Errorf("completed row missing the frozen duration clock, got %q", got)
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

// A completed fan-out's rows persist so the per-subagent cost stays visible,
// but dropFinishedParallel forgets any call whose tasks have all ended — the
// next send or run end clears them rather than leaving them stacked forever.
func TestDropFinishedParallelForgetsCompletedCalls(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["done"] = make([]parallelTaskInfo, 1)
	m.parallelTasks["done"][0] = parallelTaskInfo{Began: time.Now(), End: time.Now(), Active: false}
	m.parallelTasks["live"] = make([]parallelTaskInfo, 1)
	m.parallelTasks["live"][0] = parallelTaskInfo{Began: time.Now(), Active: true}

	m.dropFinishedParallel()

	if _, ok := m.parallelTasks["done"]; ok {
		t.Error("a fully-completed call was not forgotten")
	}
	if _, ok := m.parallelTasks["live"]; !ok {
		t.Error("a call with a running task was forgotten")
	}
}

// taskTitle prefers the short title the summarizer generated over the raw
// task prompt, so the running row stops dumping the full delegate prompt.
func TestTaskTitlePrefersTheGeneratedTitle(t *testing.T) {
	pt := parallelTaskInfo{Task: "analyze http://x.dev\nfor TODO comments and read the git log", Title: "scan site for todos", Active: true}
	got := taskTitle(pt)
	if got != "scan site for todos" {
		t.Errorf("taskTitle = %q, want the generated title", got)
	}
}

// Without a title the row falls back to the task prompt collapsed onto one
// line, preserving the pre-title behaviour.
func TestTaskTitleFallsBackToCollapsedTask(t *testing.T) {
	pt := parallelTaskInfo{Task: "analyze http://x.dev\nfor TODO comments", Active: true}
	if got := taskTitle(pt); got != "analyze http://x.dev for TODO comments" {
		t.Errorf("taskTitle = %q, want the collapsed task", got)
	}
}

// shortTitle hard-caps a title the model ran long at seven words so a row can
// never wrap.
func TestShortTitleCapsAtSevenWords(t *testing.T) {
	if got := shortTitle("a b c d e f g h i j k"); got != "a b c d e f g" {
		t.Errorf("shortTitle = %q, want the first seven words", got)
	}
	if got := shortTitle("short title"); got != "short title" {
		t.Errorf("shortTitle = %q, want it untouched when short", got)
	}
}

// recordTitle applies a summarizer result to the right task in the right call
// and re-arms the watch, so the title lands without blocking the loop.
func TestRecordTitleAppliesToTheRightTask(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["t0"] = make([]parallelTaskInfo, 2)
	m.parallelTasks["t0"][0] = parallelTaskInfo{Task: "one", Active: true}
	m.parallelTasks["t0"][1] = parallelTaskInfo{Task: "two", Active: true}

	m.recordTitle(taskTitled{Call: "t0", Index: 1, Title: "second one"})

	if m.parallelTasks["t0"][1].Title != "second one" {
		t.Errorf("title not applied to task 1")
	}
	if m.parallelTasks["t0"][0].Title != "" {
		t.Errorf("title leaked onto task 0")
	}

	m.recordTitle(taskTitled{Call: "missing", Index: 0, Title: "ghost"})
	if _, ok := m.parallelTasks["missing"]; ok {
		t.Error("a title to an unknown call created state")
	}
}
