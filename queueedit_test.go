package main

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// up and down are the presses the walk is driven by.
var (
	up   = tea.KeyPressMsg{Code: tea.KeyUp}
	down = tea.KeyPressMsg{Code: tea.KeyDown}
)

// busyWith puts a model in the state the queue only ever exists in: a run in
// flight, with lines typed behind it.
func busyWith(queued ...string) *model {
	m := sized()
	m.run.busy = true
	m.run.began = time.Now()
	m.run.cancel = func() {}
	m.run.queued = append([]string{}, queued...)
	return m
}

// The crash this was written for. recall walked the index past zero and then
// indexed the list at -1, so one question sent and two Ups took the whole
// client down — no error, no transcript, the session gone.
func TestWalkingPastTheOldestEntryDoesNotPanic(t *testing.T) {
	m := sized()
	m.remember("the only question")

	for i := range 5 {
		m.key(up)
		if m.hist.index < 0 {
			t.Fatalf("index = %d after %d ups, want it never below zero", m.hist.index, i+1)
		}
	}
	if got := m.prompt.Value(); got != "the only question" {
		t.Errorf("prompt = %q, want it parked on the oldest entry", got)
	}
}

// Up reaches the queue before it reaches anything already sent. A line still
// waiting to go out is both the most recent thing typed and the only kind that
// can still be changed, so it is what somebody pressing Up is reaching for.
func TestUpWalksTheQueueBeforeTheSentQuestions(t *testing.T) {
	m := busyWith("first queued", "second queued")
	m.remember("already sent")
	m.hist.index = len(m.walkable())

	m.key(up)
	if got := m.prompt.Value(); got != "second queued" {
		t.Fatalf("first up = %q, want the line queued most recently", got)
	}
	m.key(up)
	if got := m.prompt.Value(); got != "first queued" {
		t.Fatalf("second up = %q, want the earlier queued line", got)
	}
	m.key(up)
	if got := m.prompt.Value(); got != "already sent" {
		t.Errorf("third up = %q, want the walk to carry on into the sent questions", got)
	}
}

// Editing a queued line replaces it where it stands. Appending the edit would
// send the question twice — once as typed and once as corrected — which is the
// opposite of what fixing a typo is for.
func TestEditingAQueuedLineReplacesItRatherThanAddingAnother(t *testing.T) {
	m := busyWith("frist queued", "second queued")
	m.hist.index = len(m.walkable())

	m.key(up)
	m.key(up)
	m.setEntry("first queued")
	m.ask()

	want := []string{"first queued", "second queued"}
	if len(m.run.queued) != len(want) {
		t.Fatalf("queue = %q, want the same two lines with the first corrected", m.run.queued)
	}
	for i, line := range want {
		if m.run.queued[i] != line {
			t.Errorf("queue[%d] = %q, want %q", i, m.run.queued[i], line)
		}
	}
}

// The queue drains from the front while somebody is editing the back of it, so
// the line being edited is named by its distance from the end. Anything
// counting from the front would put the edit over a different message.
func TestAnEditSurvivesTheQueueDrainingUnderneathIt(t *testing.T) {
	m := busyWith("going out now", "still queued")
	m.hist.index = len(m.walkable())

	m.key(up)
	if got := m.prompt.Value(); got != "still queued" {
		t.Fatalf("up = %q, want the last queued line", got)
	}

	m.run.queued = m.run.queued[1:]
	m.setEntry("still queued, corrected")
	m.ask()

	if len(m.run.queued) != 1 || m.run.queued[0] != "still queued, corrected" {
		t.Errorf("queue = %q, want the one remaining line corrected in place", m.run.queued)
	}
}

// The line being edited can be sent before the edit is finished, and a sent
// question cannot be unsent. The edit becomes a new message rather than
// silently overwriting whichever line has taken its place.
func TestAnEditOfAnAlreadyDeliveredLineBecomesANewMessage(t *testing.T) {
	m := busyWith("about to go out")
	m.hist.index = len(m.walkable())

	m.key(up)
	m.run.queued = nil
	m.setEntry("too late, corrected")
	m.ask()

	if len(m.run.queued) != 1 || m.run.queued[0] != "too late, corrected" {
		t.Errorf("queue = %q, want the edit queued as its own message", m.run.queued)
	}
}

// Down comes back out of the walk to whatever was being written when it
// started, queue or no queue.
func TestDownReturnsToTheDraftThroughTheQueue(t *testing.T) {
	m := busyWith("queued line")
	m.hist.index = len(m.walkable())
	m.prompt.SetValue("half a thought")

	m.key(up)
	if got := m.prompt.Value(); got != "queued line" {
		t.Fatalf("up = %q, want the queued line", got)
	}
	m.key(down)
	if got := m.prompt.Value(); got != "half a thought" {
		t.Errorf("down = %q, want the draft back", got)
	}
	if m.hist.fromEnd != 0 {
		t.Errorf("fromEnd = %d at the draft, want the walk off the queue", m.hist.fromEnd)
	}
}

// Esc means "not this answer, move on" and ctrl+c means "stop". The queue is
// the whole difference: dropping it on esc made the queue useless exactly when
// it was most wanted, because the reader had to watch a wrong answer out to
// keep what they had typed behind it.
func TestEscStopsTheRunAndKeepsTheQueue(t *testing.T) {
	m := busyWith("the follow-up")
	m.escaped()

	if len(m.run.queued) != 1 {
		t.Fatalf("queue = %q after esc, want it still standing", m.run.queued)
	}
	if m.run.stop != abandoned {
		t.Errorf("stop = %q, want the run marked abandoned", m.run.stop)
	}
	if said := strings.Join(spoken(m), "\n"); !strings.Contains(said, "still to send") {
		t.Errorf("transcript = %q, want esc to say the queue is still going", said)
	}
}

// Ctrl+c keeps meaning stop. Left alone the queue is delivered by settle,
// which cancelling reaches like any other ending, so a stop key that kept it
// would start the next run on the spot.
func TestCtrlCStopsTheRunAndDropsTheQueue(t *testing.T) {
	m := busyWith("never sent")
	m.abandon()

	if len(m.run.queued) != 0 {
		t.Fatalf("queue = %q after ctrl+c, want it dropped", m.run.queued)
	}
	if said := strings.Join(spoken(m), "\n"); !strings.Contains(said, "dropped, not sent") {
		t.Errorf("transcript = %q, want the drop said out loud", said)
	}
}

// A line pulled into the prompt for editing is drawn in the prompt. Left in
// the queue list as well it is on screen twice — once as it is being rewritten
// and once, above, in the state it is being rewritten out of — which reads as
// the edit having failed to take.
func TestTheLineBeingEditedLeavesTheQueueOnScreen(t *testing.T) {
	m := busyWith("first queued", "second queued")
	m.hist.index = len(m.walkable())

	m.key(up)
	drawn := strings.Join(m.viewQueued(), "\n")
	if strings.Contains(visible(drawn), "second queued") {
		t.Errorf("queue = %q, want the line being edited gone from the list", visible(drawn))
	}
	if !strings.Contains(visible(drawn), "first queued") {
		t.Errorf("queue = %q, want the lines not being edited still listed", visible(drawn))
	}
	if m.queuedHeight() != len(m.viewQueued()) {
		t.Errorf("queuedHeight = %d but %d rows drawn — layout reserves what view draws",
			m.queuedHeight(), len(m.viewQueued()))
	}
}

// A run settling mid-edit must not send the version being replaced. That is
// the one wording the reader has already decided is wrong, and their edit
// would arrive after it as a near-duplicate.
func TestARunSettlingMidEditDoesNotSendTheLineBeingEdited(t *testing.T) {
	m := busyWith("goes out", "being edited")
	m.hist.index = len(m.walkable())
	m.key(up)

	if at := m.nextToSend(); at != 0 {
		t.Fatalf("nextToSend = %d, want the line that is not being edited", at)
	}
	m.run.queued = m.run.queued[1:]
	if at := m.nextToSend(); at != -1 {
		t.Errorf("nextToSend = %d, want nothing sendable while the last line is being edited", at)
	}
}
