package main

import (
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

// What a run that produced nothing has to say for itself, split from the queue
// tests because filet caps a file at 250 lines and these are a different
// subject: not what was typed, but what came back.

// A stream that yields a turn and a done and no text between them is
// well-formed: nothing errored, nothing was refused, so cutShort has nothing
// to say and the status line goes back to "ready" under a transcript holding
// only the question. Measured against openrouter/stealth-ox-alpha, which
// returns exactly this for any request carrying a tool.
func TestARunThatSaidNothingSaysSo(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.cancel = func() {}
	m.settle()

	said := strings.Join(spoken(m), "\n")
	if !strings.Contains(said, "no answer") {
		t.Errorf("transcript = %q, want an empty run reported rather than a silent 'ready'", said)
	}
}

// An abandoned run already has its own words in cutShort, and a run that said
// something needs no explaining. Two accounts of one silence is worse than
// none.
func TestARunThatSaidSomethingIsNotReportedAsEmpty(t *testing.T) {
	for _, tc := range []struct {
		name     string
		reported bool
		stop     nacelle.Stop
	}{
		{"said something", true, ""},
		{"abandoned", false, abandoned},
	} {
		m := sized()
		m.run.busy = true
		m.run.cancel = func() {}
		m.run.reported, m.run.stop = tc.reported, tc.stop
		m.settle()

		if said := strings.Join(spoken(m), "\n"); strings.Contains(said, "no answer") {
			t.Errorf("%s: transcript = %q, want no empty-run report", tc.name, said)
		}
	}
}
