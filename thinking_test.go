package main

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// thought is a model that has just finished streaming reasoning, with the
// clock stamped far enough back that the duration is a fixed number rather
// than however long the test took.
func thought(text string, ago time.Duration) *model {
	m := sized()
	m.run.reasoning.WriteString(text)
	m.begun = time.Now().Add(-ago)
	return m
}

// The transcript gets one quiet line, not the chain of thought. Printing both
// makes the answer the thing that has to be found in what preceded it, and the
// answer is what the window is for.
func TestReasoningCommitsAsOneCollapsedLine(t *testing.T) {
	m := thought("first I would have to\nand then\nand then again", 4200*time.Millisecond)

	m.flush()

	said := strings.Join(spoken(m), "\n")
	if !strings.Contains(said, "▶ thought for 4.2s") {
		t.Errorf("said = %q, want the collapsed line with its duration", said)
	}
	if strings.Contains(said, "and then again") {
		t.Errorf("said = %q, want the reasoning itself kept out of the transcript", said)
	}
}

// Tenths under ten seconds, whole seconds over: the tenth is the difference
// between answering and stopping to think, right up until it is noise on a
// figure nobody reads that closely.
func TestAThinkingDurationReadsTheWayItIsRead(t *testing.T) {
	cases := []struct {
		spent time.Duration
		want  string
	}{
		{450 * time.Millisecond, "0.5s"},
		{4200 * time.Millisecond, "4.2s"},
		{9949 * time.Millisecond, "9.9s"},
		{10 * time.Second, "10s"},
		{92500 * time.Millisecond, "93s"},
	}

	for _, c := range cases {
		if got := roughly(c.spent); got != c.want {
			t.Errorf("roughly(%s) = %q, want %q", c.spent, got, c.want)
		}
	}
}

// A duration nobody measured is printed as no duration at all, rather than as
// an invented 0.0s. Nothing stamps a start unless a frame was drawn while the
// buffer was filling.
func TestUnmeasuredThinkingReportsNoDuration(t *testing.T) {
	m := sized()
	m.run.reasoning.WriteString("some reasoning")

	m.flush()

	said := strings.Join(spoken(m), "\n")
	if !strings.Contains(said, "▶ thought") || strings.Contains(said, "for") {
		t.Errorf("said = %q, want the collapsed line with no duration on it", said)
	}
}

// The hint is how the key is discovered, and once is how often that is worth
// saying. Repeated under every answer it turns the quietest row on the screen
// into the one that repeats itself.
func TestTheExpandHintIsShownOnceAndNotAgain(t *testing.T) {
	m := thought("thinking about it", time.Second)
	m.flush()
	if first := strings.Join(spoken(m), "\n"); !strings.Contains(first, "ctrl+t to expand") {
		t.Errorf("first line = %q, want it to name the key", first)
	}

	m.unprinted = nil
	m.run.reasoning.WriteString("thinking again")
	m.begun = time.Now().Add(-time.Second)
	m.flush()

	if second := strings.Join(m.unprinted, "\n"); strings.Contains(second, "ctrl+t") {
		t.Errorf("second line = %q, want the hint shown once a session", second)
	}
}

// Collapsing may not throw the text away, or the key that expands it has
// nothing to print.
func TestTheFullReasoningIsRetainedAfterCollapsing(t *testing.T) {
	m := thought("the whole chain of thought", time.Second)

	m.flush()

	if m.retained != "the whole chain of thought" {
		t.Errorf("retained = %q, want the reasoning kept in memory", m.retained)
	}
}

// Expanding prints the retained text now, underneath. A printed line belongs
// to the terminal from the moment it is written and can never be rewritten, so
// a second copy below is the only shape "expand" has here.
func TestCtrlTPrintsTheRetainedReasoning(t *testing.T) {
	m := thought("the whole chain of thought", time.Second)
	m.flush()
	m.unprinted = nil

	handled, _ := m.key(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})

	if !handled {
		t.Fatal("ctrl+t was not claimed, want the client to own the key")
	}
	if said := visible(strings.Join(m.unprinted, "\n")); !strings.Contains(said, "the whole chain of thought") {
		t.Errorf("said = %q, want the retained reasoning printed", said)
	}
}

// The press sets a mode rather than answering one turn. Somebody who wants to
// read the model's thinking wants to read it for the session, not to press a
// key again after every answer.
func TestExpandingSticksForTheTurnsAfterIt(t *testing.T) {
	m := thought("the first turn's reasoning", time.Second)
	m.flush()
	m.key(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	m.unprinted = nil

	m.run.reasoning.WriteString("the second turn's reasoning")
	m.begun = time.Now().Add(-time.Second)
	m.flush()
	said := visible(strings.Join(m.unprinted, "\n"))

	if !strings.Contains(said, "the second turn's reasoning") {
		t.Errorf("said = %q, want the next turn shown in full without a second press", said)
	}
	if strings.Contains(said, "▶ thought for") {
		t.Errorf("said = %q, want no collapsed line while expanded", said)
	}
}

// And pressing it again puts it back, or the mode is a one-way door.
func TestPressingItAgainCollapsesAgain(t *testing.T) {
	m := thought("the first turn's reasoning", time.Second)
	m.flush()
	m.key(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	m.key(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	m.unprinted = nil

	m.run.reasoning.WriteString("the second turn's reasoning")
	m.begun = time.Now().Add(-time.Second)
	m.flush()

	if said := visible(strings.Join(m.unprinted, "\n")); !strings.Contains(said, "▶ thought for") {
		t.Errorf("said = %q, want the collapsed line back", said)
	}
}

// Idle, the key still has a real answer: the mode changed and the next turn
// will show it. A binding that silently does nothing is indistinguishable from
// a terminal that dropped the press.
func TestCtrlTWithNothingRetainedSaysSoQuietly(t *testing.T) {
	m := sized()
	m.unprinted = nil

	handled, cmd := m.key(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})

	if !handled || cmd != nil {
		t.Fatalf("handled, cmd = %v, %v; want the press claimed and nothing started", handled, cmd)
	}
	if !m.expanded {
		t.Error("expanded = false, want the mode toggled with nothing to print")
	}
	if said := visible(strings.Join(m.unprinted, "\n")); !strings.Contains(said, "reasoning will be shown in full") {
		t.Errorf("said = %q, want one line saying what the press did", said)
	}
}

// The honest start is the first thinking delta. absorb lives in a file this
// cannot reach, so the clock is stamped by the next frame — and only ever by
// the first one that finds the buffer non-empty.
func TestTheClockStartsAtTheFirstThinkingDeltaAndOnlyThen(t *testing.T) {
	m := sized()
	m.streaming()
	if !m.begun.IsZero() {
		t.Fatal("begun was stamped with nothing streaming, want no clock until there is reasoning")
	}

	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: "first"})
	m.streaming()
	first := m.begun
	if first.IsZero() {
		t.Fatal("begun is zero, want the first delta to start the clock")
	}

	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: " and more"})
	m.streaming()
	if !m.begun.Equal(first) {
		t.Errorf("begun = %v, want the first delta's stamp kept, not the latest frame's", m.begun)
	}
}
