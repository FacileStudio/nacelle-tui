package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

// sideView renders one line per side run: the question collapsed to an action
// with a live clock, and a done check once the answer has committed.
func TestSideViewRendersOneLinePerSideRun(t *testing.T) {
	m := sized()
	m.sides = []sideRun{
		{prompt: "what is the capital of france\nand its population", began: time.Now(), done: false},
	}
	got := visible(m.sideView())
	lines := strings.Split(got, "\n")
	if len(lines) != 1 {
		t.Fatalf("sideView = %q, want one row per side run", got)
	}
	if !strings.Contains(got, "⇄") {
		t.Errorf("row missing the ⇄ glyph: %q", got)
	}

	m.sides[0].done = true
	if got2 := visible(m.sideView()); !strings.Contains(got2, "✓") {
		t.Errorf("done row missing the check: %q", got2)
	}
}

// recordSide streams a reply's text and, on the done marker, folds the
// remainder into the transcript and closes the run.
func TestRecordSideCommitsStreamedText(t *testing.T) {
	m := sized()
	m.sides = []sideRun{{prompt: "q", began: time.Now()}}

	m.recordSide(sideResult{index: 0, event: nacelle.Event{Kind: nacelle.KindText, Text: "one\ntwo"}})
	if m.sides[0].done {
		t.Error("run marked done before the done marker")
	}
	if !strings.Contains(visible(strings.Join(m.unprinted, "\n")), "one") {
		t.Error("completed line 'one' not committed to the scrollback")
	}

	m.recordSide(sideResult{index: 0, event: nacelle.Event{Kind: nacelle.KindTurn, Usage: nacelle.Usage{OutputTokens: 4}}})
	m.recordSide(sideResult{index: 0, done: true})
	if !m.sides[0].done {
		t.Error("run not marked done on the done marker")
	}
	if m.sides[0].usage.OutputTokens != 4 {
		t.Errorf("usage = %v, want the turn's bill", m.sides[0].usage)
	}
}

// dropFinishedSides forgets done runs once their answers are committed, so
// their rows leave at the next send or run end.
func TestDropFinishedSidesForgetsDoneRuns(t *testing.T) {
	m := sized()
	m.sides = []sideRun{
		{prompt: "done", began: time.Now(), done: true},
		{prompt: "live", began: time.Now(), done: false},
	}
	m.dropFinishedSides()
	if len(m.sides) != 1 || m.sides[0].prompt != "live" {
		t.Errorf("dropFinishedSides left %d runs, want only the live one", len(m.sides))
	}
}

// ask during a busy run turns a plain message into a side run rather than
// queueing it, while a command still queues.
func TestAskDuringBusyStartsASideRunForAMessage(t *testing.T) {
	m := sized()
	agent, err := nacelle.New(nacelle.Config{Backend: silent{}, System: "be quiet"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	m.agent = agent
	m.system = "be quiet"
	m.run.busy = true
	m.prompt.SetValue("hello while you work")
	m.ask()
	if len(m.sides) != 1 {
		t.Errorf("expected one side run, got %d", len(m.sides))
	}
	m.sides = nil
}
