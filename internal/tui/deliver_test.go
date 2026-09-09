package tui

import (
	"strings"
	"testing"
)

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

// The related guard stops an unrelated message being swallowed as an edit, but
// a message that IS related to a stale-marked queued line could still be held
// while the agent is idle — and with no run coming there is no settle to drain
// it, so it sits in the queue as "I typed and it did not send". Idle, the
// queue holds nothing that will drain on its own, so ask must dispatch rather
// than hold.
func TestAskWhenIdleSendsEvenIfTheQueueLineLooksRelated(t *testing.T) {
	m := sized()
	m.agent = answering(t)
	m.Add("make the header sticky")
	m.hist.FromEnd = 1

	m.prompt.SetValue("make the header sticky now")
	cmd := m.ask()
	defer m.run.cancel()

	if printed := printedBy(cmd); !strings.Contains(printed, "make the header sticky now") {
		t.Errorf("printed = %q, want the message dispatched, not held in the idle queue", printed)
	}
}
