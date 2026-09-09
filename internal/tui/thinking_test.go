package tui

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
	m.Begun = time.Now().Add(-ago)
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

func TestReasoningIsShownApartAndKeptOutOfTheConversation(t *testing.T) {
	m := sized()
	m.Expanded = true
	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: "let me think"})
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "the answer"})
	m.settle()

	screen := onScreen(m)
	if !strings.Contains(screen, "let me think") || !strings.Contains(screen, "the answer") {
		t.Fatalf("screen = %q, want reasoning and answer both visible", screen)
	}
	if strings.Contains(screen, "thinkthe answer") {
		t.Errorf("screen = %q, want reasoning not run into first word", screen)
	}
	if len(m.conversation) != 1 || said(m.conversation[0]) != "the answer" {
		t.Errorf("conversation = %+v, want answer alone", m.conversation)
	}
}

// A turn is closed through turn(), not only through settle's flush(), and the
// two must agree on where the thinking line goes. flush commits thinking before
// the answer; turn used to commit the answer tail first, which dropped the
// "▶ thought for Xs" line below the text it reasoned for. Streaming draws the
// trace above the answer, so both commit paths should land the thinking line
// above it too.
func TestATurnPrintsThinkingAboveTheAnswer(t *testing.T) {
	m := thought("the reasoning", 1200 * time.Millisecond)
	m.run.answer.WriteString("the answer")

	m.turn(nacelle.Event{Usage: nacelle.Usage{InputTokens: 100, OutputTokens: 50}})

	said := strings.Join(spoken(m), "\n")
	answerAt := strings.Index(said, "the answer")
	thinkAt := strings.Index(said, "▶ thought")
	if answerAt < 0 || thinkAt < 0 {
		t.Fatalf("said = %q, want both the thought line and the answer", said)
	}
	if thinkAt >= answerAt {
		t.Errorf("said = %q, want the thinking line above the answer it reasoned for", said)
	}
}

// Expanded is the same ordering, sharpened: the last reasoning line that was
// still streaming when the turn ended must not slide under the answer either.
func TestATurnPrintsTheLastReasoningLineAboveTheAnswerWhenExpanded(t *testing.T) {
	m := thought("the last line of reasoning", 1200 * time.Millisecond)
	m.Expanded = true
	m.run.answer.WriteString("the answer")

	m.turn(nacelle.Event{Usage: nacelle.Usage{InputTokens: 100, OutputTokens: 50}})

	said := strings.Join(spoken(m), "\n")
	answerAt := strings.Index(said, "the answer")
	thinkAt := strings.Index(said, "the last line of reasoning")
	if answerAt < 0 || thinkAt < 0 {
		t.Fatalf("said = %q, want both the reasoning line and the answer", said)
	}
	if thinkAt >= answerAt {
		t.Errorf("said = %q, want the reasoning line above the answer", said)
	}
}

func TestReasoningIsOnScreenWhileItIsStillStreaming(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: "weighing it up"})

	if !strings.Contains(onScreen(m), "▶ thought") {
		t.Errorf("screen = %q, want collapsed thought line visible", onScreen(m))
	}
}

// Expanded reasoning streams through the same muted style the finished lines
// scroll back up in, not through the markdown renderer the answer uses. A
// half-finished trace full of markdown painting in as bold and code is the
// artifact the renderer leaves behind, and it must not flicker in mid-stream
// only to vanish when the line commits.
func TestExpandedStreamingReasoningIsNotMarkdownRendered(t *testing.T) {
	m := sized()
	m.Expanded = true
	m.run.busy = true
	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: "**bold** and `code` reasoning"})

	screen := onScreen(m)
	if !strings.Contains(screen, "**bold**") || !strings.Contains(screen, "`code`") {
		t.Fatalf("screen = %q, want raw thinking text, not markdown-rendered", screen)
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
	m.Begun = time.Now().Add(-time.Second)
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

	if m.Retained != "the whole chain of thought" {
		t.Errorf("retained = %q, want the reasoning kept in memory", m.Retained)
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
	m.Begun = time.Now().Add(-time.Second)
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
	m.Begun = time.Now().Add(-time.Second)
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
	if !m.Expanded {
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
	if !m.Begun.IsZero() {
		t.Fatal("begun was stamped with nothing streaming, want no clock until there is reasoning")
	}

	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: "first"})
	m.streaming()
	first := m.Begun
	if first.IsZero() {
		t.Fatal("begun is zero, want the first delta to start the clock")
	}

	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: " and more"})
	m.streaming()
	if !m.Begun.Equal(first) {
		t.Errorf("begun = %v, want the first delta's stamp kept, not the latest frame's", m.Begun)
	}
}

func TestClearingTheSessionDropsWhatCtrlTWouldReprint(t *testing.T) {
	m := bareBanner()
	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: "from the old session"})
	m.flush()

	m.clear()
	m.unprinted = nil
	m.reveal()

	for _, line := range m.unprinted {
		if strings.Contains(line, "from the old session") {
			t.Errorf("ctrl+t after /clear reprinted the cleared session: %q", line)
		}
	}
}
