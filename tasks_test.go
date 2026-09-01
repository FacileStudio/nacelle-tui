package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle"
)

// plan is a list of n steps with the given one running, which is the only
// thing every test here needs to vary.
func plan(n, active int) taskList {
	list := make(taskList, 0, n)
	for i := range n {
		status := statusTodo
		switch {
		case i < active:
			status = statusDone
		case i == active:
			status = statusActive
		}
		list = append(list, taskItem{Title: "step", Status: status})
	}
	return list
}

// layout reserves rows() lines and View draws view(), so the two disagreeing
// by one pushes the prompt off the bottom of the screen. The cap is where
// that is easiest to get wrong: the extra "and N more" line is a row layout
// has to have counted.
func TestEveryPlanDrawsExactlyTheRowsItReserved(t *testing.T) {
	for _, size := range []int{0, 1, taskRows - 1, taskRows, taskRows + 1, 40} {
		list := plan(size, size/2)
		if got, want := len(list.view(80, lipgloss.NewStyle())), list.rows(); got != want {
			t.Errorf("%d steps drew %d lines, reserved %d", size, got, want)
		}
	}
}

// Past the cap the list scrolls to the step in flight. A plan whose first
// steps are done would otherwise draw nothing but ticks, hiding the one line
// the reader is watching.
func TestALongPlanShowsTheStepInFlight(t *testing.T) {
	list := plan(20, 15)
	list[15].Title = "the running one"

	drawn := strings.Join(list.view(80, lipgloss.NewStyle()), "\n")
	if !strings.Contains(drawn, "the running one") {
		t.Errorf("the running step is off screen:\n%s", drawn)
	}
	if !strings.Contains(drawn, "15 more") {
		t.Errorf("the hidden steps are not counted:\n%s", drawn)
	}
}

// A title is written by the model, so a long one is not this client's to
// trust with the width layout reserved for it: one row per line, never
// wrapped.
func TestALongTitleIsCutToTheWidth(t *testing.T) {
	list := taskList{{Title: strings.Repeat("long ", 40), Status: statusActive}}

	line := visible(list.view(30, lipgloss.NewStyle())[0])
	if got := lipgloss.Width(line); got > 30 {
		t.Errorf("line is %d cells wide, want at most 30: %q", got, line)
	}
}

// Two steps running at once is a status display that says nothing, so it is
// refused rather than drawn. Zero is not refused: a plan whose last step has
// just finished is a legitimate final snapshot.
func TestTwoStepsInProgressAreRefusedAndNoneIsAllowed(t *testing.T) {
	both := taskList{{Title: "a", Status: statusActive}, {Title: "b", Status: statusActive}}
	if err := validate(both); err == nil {
		t.Error("two steps in progress were accepted")
	}
	if err := validate(taskList{{Title: "a", Status: statusDone}}); err != nil {
		t.Errorf("a finished plan was refused: %v", err)
	}
}

// An invented status has to come back named, because the model's only way to
// fix the call is to be told which word was wrong and which ones are right.
func TestAnUnknownStatusIsRefusedByName(t *testing.T) {
	_, err := tasksTool{}.Run(t.Context(), json.RawMessage(`{"tasks":[{"title":"a","status":"scheduled"}]}`))
	if err == nil {
		t.Fatal("an unknown status was accepted")
	}
	if !strings.Contains(err.Error(), "scheduled") || !strings.Contains(err.Error(), statusActive) {
		t.Errorf("error = %v, want it to name the bad status and the good ones", err)
	}
}

// Malformed input is reported, never panicked on: a tool error is handed back
// to the model, which can retry, and a panic takes the session with it.
func TestMalformedInputComesBackAsAnError(t *testing.T) {
	if _, err := (tasksTool{}).Run(t.Context(), json.RawMessage(`{"tasks":"all of them"}`)); err == nil {
		t.Error("a string where the list should be was accepted")
	}
}

// Run happens on the agent's goroutine while View renders on bubbletea's, so
// it must not touch model state. It sends, and only the update loop's own
// recordTasks writes what is drawn.
func TestRunReportsThePlanWithoutTouchingTheModel(t *testing.T) {
	m := sized()
	sent := make(chan error, 1)
	go func() {
		_, err := tasksTool{}.Run(context.Background(), json.RawMessage(`{"tasks":[{"title":"first","status":"in_progress"}]}`))
		sent <- err
	}()
	if err := <-sent; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(m.tasks) != 0 {
		t.Fatalf("the tool wrote %d steps into the model", len(m.tasks))
	}

	update, ok := watchTasks()().(taskUpdate)
	if !ok {
		t.Fatal("the watcher did not deliver the plan as a taskUpdate")
	}
	if cmd := m.recordTasks(update); cmd == nil {
		t.Error("recording a plan did not re-arm the watcher")
	}
	if len(m.tasks) != 1 || m.tasks[0].Title != "first" {
		t.Errorf("model plan = %v, want the reported step", m.tasks)
	}
}

// The plan tool must not be in the set localTools returns, because that set is
// what a delegate inherits, and there is one plan on one screen. This fails
// the moment somebody moves the append back into localTools where it looks
// like it belongs.
func TestThePlanToolIsMountedAfterTheDelegateTakesItsCopy(t *testing.T) {
	config := defaults()
	config.Root = t.TempDir()

	set, local, err := localTools(config)
	if set != nil {
		t.Cleanup(func() {
			if err := set.Close(); err != nil {
				t.Errorf("closing the tool set: %v", err)
			}
		})
	}
	if err != nil {
		t.Fatalf("localTools: %v", err)
	}
	if named(local, "tasks") != 0 {
		t.Errorf("localTools returned the plan tool, so a delegate inherits it")
	}
	if got := named(withTasks(local), "tasks"); got != 1 {
		t.Errorf("withTasks mounted the plan tool %d times, want exactly one", got)
	}
}

// named counts the tools called name, because "is it there" and "is it there
// twice" are the two ways the mounting can be wrong.
func named(local []nacelle.Tool, name string) int {
	count := 0
	for _, tool := range local {
		if tool.Name() == name {
			count++
		}
	}
	return count
}

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
	// currentPlan starts as zero — nothing has been stored yet.
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
