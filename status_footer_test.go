package main

import (
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

// The reason the words move at all: a spinner proves the program is alive,
// not that the wait is going anywhere, and one fixed sentence under it reads
// as a frozen line to anyone who looks away and back.
func TestTheWaitingWordsChangeWhileNothingArrives(t *testing.T) {
	elapsed := time.Duration(0)
	first := waitingVerb(elapsed)

	if next := waitingVerb(elapsed + rephrase); next == first {
		t.Errorf("the line still says %q one rephrase later, so a long wait never changes", next)
	}
	if round := waitingVerb(elapsed + time.Duration(len(waiting))*rephrase); round != first {
		t.Errorf("waitingVerb = %q after a full cycle, want it back at %q", round, first)
	}
}

// A phrase is picked from the clock rather than from how long this wait has
// lasted, so any phrasing implying duration — "still waiting" — would be a
// lie two hundred milliseconds in. Every one of them has to read as true at
// the first frame of a run.
func TestEveryWaitingPhraseIsTrueTheMomentAskingStarts(t *testing.T) {
	for _, phrase := range waiting {
		if !strings.HasPrefix(phrase, "waiting ") {
			t.Errorf("waiting phrase %q claims more than that nothing has come back yet", phrase)
		}
	}
}

// The status row is the only proof the client is still doing something, and it
// was drawn in the same grey as everything that had already finished — on the
// terminals where a scheme parks ANSI 8 near the background, the busiest line
// on screen was the hardest one to read. It carries a hue of its own now, and
// the spinner carries the same one: a neutral spinner beside a coloured phrase
// reads as two widgets sharing a row rather than one sentence about one run.
func TestTheSpinnerAndTheWaitingPhraseAreOneColouredStatement(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.began = time.Now()

	line, _, _ := strings.Cut(m.status(), "\n")
	if strings.HasPrefix(line, visible(line)) {
		t.Fatalf("status = %q, want the whole line coloured rather than left plain", line)
	}
	if opens := strings.Index(line, "\x1b["); opens != 0 {
		t.Errorf("status = %q, want the colour to open before the spinner, not after it", line)
	}
	if inner := strings.Count(line, "\x1b["); inner != 2 {
		t.Errorf("status = %q, want one colour and one reset, not %d sequences", line, inner)
	}
}

// Which colour is the phase, so the frame where it changes is itself the
// statement that waiting turned into running. An MCP tool has no glyph of its
// own and used to fall back to no colour at all, which made a call in flight
// look like a run that had stopped doing anything.
func TestTheStatusColourSaysWhichPhaseTheRunIsIn(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.began = time.Now()
	idle := m.status()

	for _, name := range []string{"run_command", "some_mcp_tool"} {
		m.run.beginTool(nacelle.ToolEvent{ID: "1", Name: name, Input: `{}`}, false)
		busy := m.status()

		if colourOf(busy) == "" {
			t.Errorf("status running %q = %q, want a colour rather than a plain line", name, busy)
		}
		if colourOf(busy) == colourOf(idle) {
			t.Errorf("status running %q wears the waiting colour, so the phase change is invisible", name)
		}
	}
}

// colourOf is the escape sequence a rendered line opens with, or the empty
// string for one that opens with text.
func colourOf(line string) string {
	if !strings.HasPrefix(line, "\x1b[") {
		return ""
	}
	sequence, _, _ := strings.Cut(line, "m")
	return sequence
}

// Cost is the figure someone is answerable for and the one a backend only
// sometimes reports, so it has to survive a terminal narrow enough to cut the
// counts it summarises.
func TestCostIsShownAheadOfTheTokenCounts(t *testing.T) {
	m := sized()
	m.spent = nacelle.Usage{InputTokens: 2600, OutputTokens: 1100, Cost: 0.0123}

	status := visible(m.status())
	if !strings.Contains(status, "$0.0123") {
		t.Fatalf("status = %q, want the reported cost in it", status)
	}
	if strings.Index(status, "$0.0123") > strings.Index(status, "in 2.6k") {
		t.Errorf("status = %q, want the cost ahead of the counts it summarises", status)
	}
}

// Anthropic reports tokens and no cost at all, and a $0.0000 beside a
// currency symbol reads as free rather than as unknown.
func TestAnUnreportedCostIsLeftOutRatherThanShownAsZero(t *testing.T) {
	m := sized()
	m.spent = nacelle.Usage{InputTokens: 2600, OutputTokens: 1100}

	if status := visible(m.status()); strings.Contains(status, "$") {
		t.Errorf("status = %q, want no currency figure when the backend reported none", status)
	}
}

// TestTheElapsedTimerMeasuresTheRunAndDisappearsWithIt pins the two halves of
// the timer that are easy to get backwards: it counts from the moment this run
// started rather than from the moment the client did, and an idle line has no
// timer at all rather than one frozen at whatever the last run reached.
func TestTheElapsedTimerMeasuresTheRunAndDisappearsWithIt(t *testing.T) {
	m := newModel(nil, "banner", nil)
	m.began = time.Now().Add(-41 * time.Minute)
	m.run.began = time.Now().Add(-12 * time.Second)
	m.run.busy = true

	if got := m.status(); !strings.Contains(got, "12s") {
		t.Errorf("status while running = %q, want the run's own 12s", got)
	}
	if got := m.status(); strings.Contains(got, "41m") {
		t.Errorf("status while running = %q, want the run's span and not the session's", got)
	}

	m.run.busy = false
	if got := m.status(); strings.Contains(got, "12s") {
		t.Errorf("idle status = %q, want no timer left standing", got)
	}
}

// TestAStatusLineDrawnWithoutSendIsNotGivenAThousandHourTimer covers the one
// way the busy flag and the stamp can disagree: a test drawing a status line
// sets busy by hand and never reaches send, and time.Since on a zero Time is a
// span in the thousands of hours.
func TestAStatusLineDrawnWithoutSendIsNotGivenAThousandHourTimer(t *testing.T) {
	m := newModel(nil, "banner", nil)
	m.run.busy = true

	if got := m.ongoing(); got != "" {
		t.Errorf("ongoing with no stamp = %q, want the empty string", got)
	}
}
