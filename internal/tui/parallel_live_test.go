package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FacileStudio/nacelle"
)

// The tests for the parallel fan-out's live-update channel and the tick guard
// live here rather than in parallel_result_test.go, which is at filet's line
// cap. The coverage itself mirrors the code split: result.go owns the settled
// rows, this file owns what moves while a task runs.

// recordUpdate folds a streamed spend into its running task's total, so the row
// shows the token counter moving before the result arrives.
func TestRecordUpdateFoldsSpend(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["d0"] = make([]parallelTaskInfo, 1)
	m.parallelTasks["d0"][0] = parallelTaskInfo{Task: "one", Active: true}

	m.recordUpdate(subagentUpdate{batch: "d0", idx: 0, usage: nacelle.Usage{InputTokens: 10, OutputTokens: 2}, spend: true})
	m.recordUpdate(subagentUpdate{batch: "d0", idx: 0, usage: nacelle.Usage{InputTokens: 5}, spend: true})

	pt := &m.parallelTasks["d0"][0]
	if pt.Usage.InputTokens != 15 || pt.Usage.OutputTokens != 2 {
		t.Errorf("live usage not accumulated into the running task: %+v", pt.Usage)
	}

	m.recordUpdate(subagentUpdate{batch: "ghost", idx: 0, usage: nacelle.Usage{InputTokens: 1}, spend: true})
	if _, ok := m.parallelTasks["ghost"]; ok {
		t.Error("a usage event to an unknown batch created state")
	}
}

// A live spend that trails a finished result must not double-count a row the
// result already settled.
func TestRecordUpdateIgnoresFinishedSpend(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["d0"] = make([]parallelTaskInfo, 1)
	m.parallelTasks["d0"][0] = parallelTaskInfo{Task: "one", Active: false, Usage: nacelle.Usage{InputTokens: 100}}

	m.recordUpdate(subagentUpdate{batch: "d0", idx: 0, usage: nacelle.Usage{InputTokens: 5}, spend: true})

	if got := m.parallelTasks["d0"][0].Usage.InputTokens; got != 100 {
		t.Errorf("a late usage event double-counted a finished task: input = %d, want 100", got)
	}
}

// shortTitle also drops path and URL tokens, so a title that names a location
// reads as a plain description instead of the path the task happened to use —
// the sin the summarizer prompt must not repeat after this guard is bypassed.
func TestShortTitleStripsPathsAndURLs(t *testing.T) {
	if got := shortTitle("inline the include file at /src/main.go now"); got != "inline the include file at now" {
		t.Errorf("shortTitle path strip = %q, want the path dropped", got)
	}
	if got := shortTitle("fetch https://example.com pricing and diff"); got != "fetch pricing and diff" {
		t.Errorf("shortTitle URL strip = %q, want the URL dropped", got)
	}
}

// hasLiveParallel is what keeps the spinner ticking once a fan-out's parent run
// has settled, so it has to agree with the task rows themselves.
func TestHasLiveParallel(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	if m.hasLiveParallel() {
		t.Error("no tasks yet, hasLiveParallel true")
	}
	m.parallelTasks["done"] = make([]parallelTaskInfo, 1)
	m.parallelTasks["done"][0] = parallelTaskInfo{Active: false}
	if m.hasLiveParallel() {
		t.Error("only finished tasks, hasLiveParallel true")
	}
	m.parallelTasks["live"] = make([]parallelTaskInfo, 1)
	m.parallelTasks["live"][0] = parallelTaskInfo{Active: true}
	if !m.hasLiveParallel() {
		t.Error("a running task, hasLiveParallel false")
	}
}

// A loop that keeps the live substream ticking only while a task is still
// running redraws the elapsed clock; a finished fan-out should stop waking the
// program on its own. spun is where that decision lives.
func TestSpunKeepsTickingUntilNoLiveParallels(t *testing.T) {
	m := sized()
	m.run.busy = false
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["d0"] = make([]parallelTaskInfo, 1)
	m.parallelTasks["d0"][0] = parallelTaskInfo{Active: true}

	msg, ok := m.spin.Tick().(spinner.TickMsg)
	if !ok {
		t.Fatal("Tick did not produce a spinner.TickMsg")
	}
	if cmd := m.spun(msg); cmd == nil {
		t.Error("spun stopped ticking while a task is still running")
	}

	m.parallelTasks["d0"][0].Active = false
	if cmd := m.spun(msg); cmd != nil {
		t.Error("spun kept ticking after every task finished")
	}
}

// /clear hides finished subagent rows while leaving still-running ones in
// place — and because a running sibling's live updates and result address its
// slice position, the finished task stays in that slice (Cleared) rather than
// being cut out from under it.
func TestClearFinishedParallelKeepsRunningTasks(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["d0"] = []parallelTaskInfo{
		{Task: "one", Active: true},
		{Task: "two", Active: false, Result: "done"},
	}
	m.parallelTasks["gone"] = []parallelTaskInfo{
		{Task: "x", Active: false, Result: "done"},
	}

	m.clearFinishedParallel()

	if _, ok := m.parallelTasks["gone"]; ok {
		t.Error("a batch left with no running task survived /clear")
	}
	d0, ok := m.parallelTasks["d0"]
	if !ok {
		t.Fatal("a batch with a running task was forgotten by /clear")
	}
	if d0[0].Active == false || d0[0].Cleared {
		t.Errorf("running task disturbed by /clear: active=%v cleared=%v", d0[0].Active, d0[0].Cleared)
	}
	if d0[1].Active != false || d0[1].Cleared == false {
		t.Errorf("finished task not hidden by /clear: active=%v cleared=%v", d0[1].Active, d0[1].Cleared)
	}
	if len(d0) != 2 {
		t.Errorf("slice compacted by /clear (len %d), breaking live-update indices", len(d0))
	}
}

// A cleared task does not draw a row and does not reserve layout space, so the
// rows under the prompt match what the view reserves.
func TestClearedTasksAreNotDrawn(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["d0"] = []parallelTaskInfo{
		{Task: "hidden", Cleared: true},
		{Task: "visible", Active: true},
	}

	got := visible(m.parallelTasksView())
	if strings.Contains(got, "hidden") || !strings.Contains(got, "visible") {
		t.Errorf("view = %q, want only the running task drawn", got)
	}
	if rows := parallelTaskRows(m.parallelTasks); rows != 1 {
		t.Errorf("parallelTaskRows = %d, want 1 (cleared tasks don't reserve a row)", rows)
	}
}

// A row is laid out at the width margined shaves to, so its timer tail
// survives the margin pass instead of being clipped off the right edge.
func TestTaskRowFitsInsideMargin(t *testing.T) {
	m := sized()
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["d0"] = []parallelTaskInfo{
		{Task: "a long running task title that could push the tail", Active: true, Began: time.Now().Add(-time.Minute)},
		{Task: "done with spend", Usage: nacelle.Usage{Cost: 0.0123, OutputTokens: 9000}, Began: time.Now().Add(-time.Minute), End: time.Now()},
	}
	for _, row := range strings.Split(m.parallelTasksView(), "\n") {
		if w := lipgloss.Width(row); w > m.width-2 {
			t.Errorf("row width %d exceeds the shaved width %d, tail gets clipped: %q", w, m.width-2, ansi.Strip(row))
		}
	}
}
