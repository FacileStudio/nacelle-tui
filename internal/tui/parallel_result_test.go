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
// line, minus path and URL tokens, capped short so a fan-out whose summarizer
// has not landed never reads as the full prompt or a file location.
func TestTaskTitleFallsBackToCollapsedTask(t *testing.T) {
	pt := parallelTaskInfo{Task: "analyze http://x.dev\nfor TODO comments", Active: true}
	if got := taskTitle(pt); got != "analyze for TODO comments" {
		t.Errorf("taskTitle = %q, want the collapsed task with the URL dropped", got)
	}
}

// The fallback is shortTitle-capped at seven words just like a generated title,
// so a long prompt without a summarizer result still stays one line.
func TestTaskTitleFallbackIsCappedShort(t *testing.T) {
	pt := parallelTaskInfo{Task: "do a b c d e f g h i j k l m n o p q r s t u v w x y z now", Active: true}
	if got := taskTitle(pt); got != "do a b c d e f" {
		t.Errorf("taskTitle = %q, want the fallback capped at seven words", got)
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

// recordUpdate applies a nested task's live tool call to its row and re-arms the
// watch, so the row shows what the subagent is running while it grinds.
func TestRecordUpdateAppliesTheRunningTool(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["d0"] = make([]parallelTaskInfo, 2)
	m.parallelTasks["d0"][0] = parallelTaskInfo{Task: "one", Active: true}
	m.parallelTasks["d0"][1] = parallelTaskInfo{Task: "two", Active: true}

	m.recordUpdate(subagentUpdate{batch: "d0", idx: 1, tool: "read_file"})

	if m.parallelTasks["d0"][1].Tool != "read_file" {
		t.Errorf("running tool not applied to task 1")
	}
	if m.parallelTasks["d0"][0].Tool != "" {
		t.Errorf("running tool leaked onto task 0")
	}

	m.recordUpdate(subagentUpdate{batch: "ghost", idx: 0, tool: "run_command"})
	if _, ok := m.parallelTasks["ghost"]; ok {
		t.Error("a tool call to an unknown batch created state")
	}
}

// A tool event that trails a finished result does not resurrect a row:
// recordUpdate only touches still-running tasks.
func TestRecordUpdateIgnoresFinishedTool(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["d0"] = make([]parallelTaskInfo, 1)
	m.parallelTasks["d0"][0] = parallelTaskInfo{Task: "one", Active: false, Tool: ""}

	m.recordUpdate(subagentUpdate{batch: "d0", idx: 0, tool: "edit_file"})

	if m.parallelTasks["d0"][0].Tool != "" {
		t.Errorf("a late tool call reset a finished task's row")
	}
}

// The row shows the subagent's currently-running tool after the title's colon,
// coloured by the tool's own rules, with the short summary as the title.
func TestParallelTaskRowShowsTheRunningTool(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["t0"] = make([]parallelTaskInfo, 1)
	m.parallelTasks["t0"][0] = parallelTaskInfo{
		Task:   "analyze the codebase for todos",
		Title:  "scan site for todos",
		Tool:   "run_command",
		Began:  time.Now(),
		Active: true,
	}

	raw := m.taskRow(m.parallelTasks["t0"][0])
	got := visible(raw)

	if !strings.Contains(got, "≫ scan site for todos:") {
		t.Errorf("row = %q, want the short title followed by a colon", got)
	}
	if !strings.Contains(got, "run_command") {
		t.Errorf("row = %q, want the running tool name", got)
	}
	if !strings.Contains(raw, "\x1b[35mrun_command") {
		t.Errorf("running tool not coloured with its tool tone: %q", raw)
	}
}

// finishDetached clears the tool column, so a completed row reads as done
// rather than still running whatever it was doing when it finished.
func TestFinishDetachedDropsTheRunningTool(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["d0"] = make([]parallelTaskInfo, 1)
	m.parallelTasks["d0"][0] = parallelTaskInfo{Task: "one", Tool: "read_file", Active: true}

	m.finishDetached(&m.parallelTasks["d0"][0], detachedResult{batch: "d0", idx: 0, result: "r"})

	pt := &m.parallelTasks["d0"][0]
	if pt.Tool != "" || pt.Active {
		t.Errorf("finished task still shows tool=%q active=%v", pt.Tool, pt.Active)
	}
}
