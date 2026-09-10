package tui

import (
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

// The completion wake-up tests live here, next to parallel_review.go and away
// from parallel_live_test.go which is at filet's line cap.

// parallelReview gates the wake-up: it fires only when nothing is running, the
// main run is idle, and at least one task produced a real result. A fan-out
// that only failed, or whose results a /clear hid, is not woken for review.
func TestParallelReviewGatesTheWakeUp(t *testing.T) {
	done := func(result string) parallelTaskInfo {
		return parallelTaskInfo{Result: result, Active: false}
	}

	m := sized()
	m.parallelTasks = map[string][]parallelTaskInfo{"d0": {done("work")}}
	if !m.parallelReview() {
		t.Error("idle fan-out with a result should trigger a review")
	}

	m.parallelTasks["d0"][0].Active = true
	if m.parallelReview() {
		t.Error("a still-running task should not trigger a review")
	}
	m.parallelTasks["d0"][0].Active = false

	m.parallelTasks["d0"][0].Result = ""
	m.parallelTasks["d0"][0].Err = "boom"
	if m.parallelReview() {
		t.Error("a fan-out that only errored should not trigger a review")
	}
	m.parallelTasks["d0"][0].Err = ""
	m.parallelTasks["d0"][0].Result = "work"

	m.run.busy = true
	if m.parallelReview() {
		t.Error("a busy main run should not trigger a review")
	}
	m.run.busy = false

	m.parallelTasks["d0"][0].Cleared = true
	if m.parallelReview() {
		t.Error("a result a /clear hid should not trigger a review")
	}
}

// reviewText hands the main agent one line per finished, un-cleared subagent —
// its title and outcome — so the review it is asked for has the work in front
// of it.
func TestReviewTextInlinesTheWork(t *testing.T) {
	m := sized()
	m.parallelTasks = map[string][]parallelTaskInfo{"d0": {
		{Task: "scrape the pricing page", Title: "scrape pricing page", Result: "prices are $10", Active: false},
		{Task: "old", Result: "stale", Active: false, Cleared: true},
		{Task: "read the log", Title: "read the log", Err: "permission denied", Active: false},
	}}

	got := m.reviewText()
	if !strings.Contains(got, "Review their work") {
		t.Errorf("review = %q, want the review instruction", got)
	}
	if !strings.Contains(got, "scrape pricing page") || !strings.Contains(got, "prices are $10") {
		t.Errorf("review = %q, want the successful task and its result", got)
	}
	if !strings.Contains(got, "read the log") || !strings.Contains(got, "errored: permission denied") {
		t.Errorf("review = %q, want the failed task and its error", got)
	}
	if strings.Contains(got, "stale") {
		t.Errorf("review = %q, must not include a cleared task's result", got)
	}
}

// When the last subagent lands while the main run is idle, recordDetached wakes
// the main agent with the fan-out's work: the review goes onto the conversation
// and a run starts.
func TestRecordDetachedWakesTheMainAgentOnCompletion(t *testing.T) {
	m := sized()
	m.agent = answering(t)
	m.parallelTasks = make(map[string][]parallelTaskInfo)
	m.parallelTasks["d0"] = []parallelTaskInfo{{Task: "count the rows", Active: true}}

	m.recordDetached(detachedResult{batch: "d0", idx: 0, result: "counted 42"})
	defer m.run.cancel()

	if !m.run.busy {
		t.Fatal("the main agent was not woken when the fan-out completed")
	}
	if m.hasLiveParallel() {
		t.Error("woken run left a live parallel row behind")
	}
	var text string
	if last := m.conversation[len(m.conversation)-1]; len(last.Parts) > 0 {
		if tx, ok := last.Parts[0].(nacelle.Text); ok {
			text = tx.Text
		}
	}
	if !strings.Contains(text, "Review their work") || !strings.Contains(text, "counted 42") {
		t.Errorf("wake-up message = %q, want the review of the completed work", text)
	}
}
