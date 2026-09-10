package tui

import (
	"strings"
	"testing"
	"time"

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
	m := thought("the reasoning", 1200*time.Millisecond)
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

// The case the endpoint test above cannot cover: a real answer streams into
// the scrollback paragraph by paragraph as it arrives, and each completed
// paragraph is printed before the turn ends. A thinking line deferred to turn()
// therefore lands *under* all of it — the paragraphs were already handed to the
// terminal. It has to be introduced the moment the answer starts, not at the
// end, or it is permanently stuck below the text it preceded. That is the bug
// the simple fixture above cannot see, because it keeps the whole answer in
// the buffer until turn().
func TestStreamedAnswerDoesNotPushTheThinkingLineBelowIt(t *testing.T) {
	m := sized()
	m.Begun = time.Now().Add(-1200 * time.Millisecond)
	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: "the reasoning"})
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "first paragraph\n"})
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "second paragraph\n"})
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "the tail"})
	m.turn(nacelle.Event{Usage: nacelle.Usage{InputTokens: 100, OutputTokens: 50}})

	said := strings.Join(spoken(m), "\n")
	answerAt := strings.Index(said, "first paragraph")
	thinkAt := strings.Index(said, "▶ thought for 1.2s")
	if answerAt < 0 || thinkAt < 0 {
		t.Fatalf("said = %q, want the thought line and the streamed answer", said)
	}
	if thinkAt >= answerAt {
		t.Errorf("said = %q, want the thinking line above the answer it reasoned for", said)
	}
}

// Expanded is the same ordering, sharpened: the last reasoning line that was
// still streaming when the answer arrived must not slide under the answer's
// already-committed paragraphs either.
func TestAnswerDoesNotPushTheLastReasoningLineBelowItWhenExpanded(t *testing.T) {
	m := sized()
	m.Expanded = true
	m.Begun = time.Now().Add(-1200 * time.Millisecond)
	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: "the last line of reasoning"})
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "first paragraph\n"})
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "the tail"})
	m.turn(nacelle.Event{Usage: nacelle.Usage{InputTokens: 100, OutputTokens: 50}})

	said := strings.Join(spoken(m), "\n")
	answerAt := strings.Index(said, "first paragraph")
	thinkAt := strings.Index(said, "the last line of reasoning")
	if answerAt < 0 || thinkAt < 0 {
		t.Fatalf("said = %q, want the reasoning line and the streamed answer", said)
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
