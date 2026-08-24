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
	at := time.Unix(0, 0)
	first := waitingVerb(at)

	if next := waitingVerb(at.Add(rephrase)); next == first {
		t.Errorf("the line still says %q one rephrase later, so a long wait never changes", next)
	}
	if round := waitingVerb(at.Add(time.Duration(len(waiting)) * rephrase)); round != first {
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
