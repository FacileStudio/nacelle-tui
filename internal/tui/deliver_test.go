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

// While a parallel_agents fan-out is running the parent is busy (it is
// mid-turn, waiting on the merged tool result), so a typed message must queue
// like any other busy-run message — not answer out of band and not go to a
// subagent. When the fan-out's turn settles, the queued line is delivered as a
// fresh parent run, i.e. processed by the main thread.
func TestMessageDuringParallelFanOutQueuesThenSendsAfterSettle(t *testing.T) {
	m := sized()
	m.agent = answering(t)
	m.run.busy = true
	m.parallelTasks = map[string][]parallelTaskInfo{
		"call1": {{Task: "task A", Active: true}},
	}

	m.prompt.SetValue("follow up while the fan-out runs")
	m.ask()

	if !m.run.busy {
		t.Fatal("the parent should stay busy through the fan-out")
	}
	if len(m.parallelTasks) != 1 {
		t.Fatalf("parallelTasks = %d, want the in-flight fan-out untouched", len(m.parallelTasks))
	}
	if m.Len() != 1 {
		t.Fatalf("queue = %d, want the follow-up queued", m.Len())
	}
	if len(m.conversation) != 0 {
		t.Fatalf("conversation = %d, want the queued message withheld until the run settles", len(m.conversation))
	}

	m.settle()
	defer m.run.cancel()

	if m.Len() != 0 {
		t.Fatalf("queue = %d, want the follow-up drained by settle", m.Len())
	}
	if !m.run.busy {
		t.Fatal("settle did not start a new parent run for the follow-up")
	}
	if len(m.conversation) != 1 {
		t.Fatalf("conversation = %d, want the follow-up appended to the main conversation", len(m.conversation))
	}
}
