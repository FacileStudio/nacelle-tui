package tasks

import (
	"encoding/json"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
		if got, want := len(list.View(80, lipgloss.NewStyle())), list.Rows(); got != want {
			t.Errorf("%d steps drew %d lines, reserved %d", size, got, want)
		}
	}
}

// A long plan shows all steps (no scrolling, no hidden count).
func TestALongPlanShowsAllSteps(t *testing.T) {
	list := plan(20, 15)
	list[15].Title = "the running one"

	drawn := strings.Join(list.View(80, lipgloss.NewStyle()), "\n")
	if !strings.Contains(drawn, "the running one") {
		t.Errorf("the running step is off screen:\n%s", drawn)
	}
	if strings.Contains(drawn, "… and") {
		t.Errorf("unexpected hidden count line found:\n%s", drawn)
	}
}

// A title is written by the model, so a long one is not this client's to
// trust with the width layout reserved for it: one row per line, never
// wrapped.
func TestALongTitleIsCutToTheWidth(t *testing.T) {
	list := taskList{{Title: strings.Repeat("long ", 40), Status: statusActive}}

	line := ansi.Strip(list.View(30, lipgloss.NewStyle())[0])
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
