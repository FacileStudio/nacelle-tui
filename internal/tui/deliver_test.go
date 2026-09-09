package tui

import "testing"

// The delivery loop skips a queued line it believes is being edited, so a
// settle does not send half a rewrite. That skip must be released the moment
// the user is not in the prompt: a FromEnd marker left by browsing history
// points at a line the user stopped editing, and holding it there strands the
// line in the ready state — the reported "I cannot send a message, it gets
// queued, not sent, though the agent has finished".
func TestASettleReleasesAStaleEditMarkOnTheQueue(t *testing.T) {
	m := sized()
	m.agent = answering(t)
	m.Add("q1")
	m.hist.FromEnd = 1

	m.settle()
	defer m.run.cancel()

	if m.Len() != 0 {
		t.Errorf("len = %d, want the stale-marked line drained by settle", m.Len())
	}
	if !m.run.busy {
		t.Error("deliver did not start a run for the released line")
	}
}
