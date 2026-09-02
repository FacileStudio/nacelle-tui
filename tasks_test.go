package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The reply names the step in flight and mentions blocked or failed counts
// when there are any, so the model knows what still needs attention.
func TestTheReplyNamesTheStepInFlight(t *testing.T) {
	got := summarise(taskList{{Title: "done", Status: statusDone}, {Title: "writing tests", Status: statusActive}})
	if !strings.Contains(got, "writing tests") || !strings.Contains(got, "1 done") {
		t.Errorf("reply = %q, want the running step and the count", got)
	}
}

// A blocked step and an in_progress step together is a legitimate plan — the
// model knows one is waiting and is working on another.
func TestBlockedStepIsAccepted(t *testing.T) {
	list := taskList{
		{Title: "setup", Status: statusDone},
		{Title: "waiting for review", Status: statusBlocked, Reason: "need approval"},
		{Title: "build", Status: statusActive},
	}
	if err := validate(list); err != nil {
		t.Fatalf("a blocked step was refused: %v", err)
	}
}

// A failed step and an in_progress step together is legitimate — the model
// has acknowledged a failure, adjusted, and moved on.
func TestFailedStepIsAccepted(t *testing.T) {
	list := taskList{
		{Title: "try approach A", Status: statusFailed, Reason: "does not scale"},
		{Title: "try approach B", Status: statusActive},
	}
	if err := validate(list); err != nil {
		t.Fatalf("a failed step was refused: %v", err)
	}
}

// A plan with only blocked and failed steps and no in_progress is accepted —
// the model may need to rethink before it can mark anything running.
func TestAllBlockedAndFailedIsAccepted(t *testing.T) {
	list := taskList{
		{Title: "step", Status: statusBlocked, Reason: "waiting"},
		{Title: "step", Status: statusFailed, Reason: "dead end"},
	}
	if err := validate(list); err != nil {
		t.Fatalf("all steps blocked or failed was refused: %v", err)
	}
}

// summarise reports blocked and failed counts when they are present.
func TestSummariseMentionsBlockedAndFailed(t *testing.T) {
	list := taskList{
		{Title: "done", Status: statusDone},
		{Title: "stuck", Status: statusBlocked, Reason: "waiting on API key"},
		{Title: "broken", Status: statusFailed, Reason: "wrong approach"},
		{Title: "working", Status: statusActive},
	}
	got := summarise(list)
	if !strings.Contains(got, "1 blocked") || !strings.Contains(got, "1 failed") {
		t.Errorf("summarise = %q, want blocked and failed counts", got)
	}
	if !strings.Contains(got, "working") {
		t.Errorf("summarise = %q, want the running step", got)
	}
}

// The glyph for a failed step is visually distinct from the other statuses.
func TestFailedStepGlyphIsDistinct(t *testing.T) {
	if g := taskGlyph(statusFailed); g == taskGlyph(statusDone) || g == taskGlyph(statusActive) || g == taskGlyph(statusTodo) || g == taskGlyph(statusBlocked) {
		t.Errorf("failed glyph %q collides with another status", g)
	}
}

// step_update changes one step by index without sending the full list.
func TestStepUpdateChangesOneStep(t *testing.T) {
	currentPlan.Store(taskList{
		{Title: "step one", Status: statusTodo},
		{Title: "step two", Status: statusActive},
	})

	result, err := tasksTool{}.Run(t.Context(), json.RawMessage(`{"step_update":{"index":0,"status":"completed"}}`))
	if err != nil {
		t.Fatalf("step_update was refused: %v", err)
	}
	if !strings.Contains(result, "1 done") || !strings.Contains(result, "step two") {
		t.Errorf("reply = %q, want the done count and running step", result)
	}

	updated := currentPlan.Load().(taskList)
	if updated[0].Status != statusDone {
		t.Errorf("step 0 status = %q, want %q", updated[0].Status, statusDone)
	}
	if updated[1].Title != "step two" {
		t.Errorf("step 1 title changed to %q", updated[1].Title)
	}
	currentPlan.Store(taskList{})
}

// step_update with no previous plan is an error.
func TestStepUpdateFailsWithoutExistingPlan(t *testing.T) {
	_, err := tasksTool{}.Run(t.Context(), json.RawMessage(`{"step_update":{"index":0,"status":"completed"}}`))
	if err == nil {
		t.Fatal("step_update with no existing plan was accepted")
	}
	if !strings.Contains(err.Error(), "existing plan") {
		t.Errorf("error = %v, want it to mention the missing plan", err)
	}
}

// step_update with an out-of-range index is an error.
func TestStepUpdateOutOfRangeIsRefused(t *testing.T) {
	currentPlan.Store(taskList{
		{Title: "only one", Status: statusTodo},
	})
	_, err := tasksTool{}.Run(t.Context(), json.RawMessage(`{"step_update":{"index":5,"status":"completed"}}`))
	if err == nil {
		t.Fatal("step_update with out-of-range index was accepted")
	}
	currentPlan.Store(taskList{})
}

// step_update can change multiple fields at once: status, title and reason.
func TestStepUpdateChangesMultipleFields(t *testing.T) {
	currentPlan.Store(taskList{
		{Title: "original", Status: statusActive, Reason: ""},
	})
	_, err := tasksTool{}.Run(t.Context(), json.RawMessage(`{"step_update":{"index":0,"status":"blocked","title":"waiting now","reason":"API unavailable"}}`))
	if err != nil {
		t.Fatalf("step_update was refused: %v", err)
	}
	updated := currentPlan.Load().(taskList)
	if updated[0].Status != statusBlocked || updated[0].Title != "waiting now" || updated[0].Reason != "API unavailable" {
		t.Errorf("step after update = %+v", updated[0])
	}
	currentPlan.Store(taskList{})
}

// A call with neither tasks nor step_update is an error.
func TestEmptyInputIsRefused(t *testing.T) {
	_, err := tasksTool{}.Run(t.Context(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("empty input was accepted")
	}
}
