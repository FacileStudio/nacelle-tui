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
// by one pushes the prompt off the bottom of the screen.
func TestEveryPlanDrawsExactlyTheRowsItReserved(t *testing.T) {
	for _, size := range []int{0, 1, 4, 5, 6, 40} {
		list := plan(size, size/2)
		if got, want := len(list.view(80, lipgloss.NewStyle())), list.rows(); got != want {
			t.Errorf("%d steps drew %d lines, reserved %d", size, got, want)
		}
	}
}

// A long plan shows all steps (no scrolling, no hidden count).
func TestALongPlanShowsAllSteps(t *testing.T) {
	list := plan(20, 15)
	list[15].Title = "the running one"

	drawn := strings.Join(list.view(80, lipgloss.NewStyle()), "\n")
	if !strings.Contains(drawn, "the running one") {
		t.Errorf("the running step is off screen:\n%s", drawn)
	}
	// Ensure no "... and N more" line appears (we show all steps)
	if strings.Contains(drawn, "… and") {
		t.Errorf("unexpected hidden count line found:\n%s", drawn)
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
	if got := named(withTasks(config, local), "tasks"); got != 1 {
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
