package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// A tool's line is held until its result so the duration can fold into it, and
// the status line names the tool while it is in flight — but the line itself
// still has to reach the screen the moment it is said, ahead of the wait for
// whatever comes next. Update flushing the queue before the blocking command
// is what guarantees that, and it is the half of this that is still load
// bearing.
// The closed channel is not scenery: reading what consume printed also runs
// the wait it re-armed, and a nil one never sends.
func TestAToolLineIsPrintedAheadOfTheWaitForTheNextEvent(t *testing.T) {
	m := sized()
	m.run.busy = true

	ended := make(chan result)
	close(ended)
	m.run.results = ended

	m.Update(result{event: nacelle.Event{
		Kind: nacelle.KindToolCall,
		Tool: &nacelle.ToolEvent{ID: "1", Name: "run_command"},
	}})
	_, cmd := m.Update(result{event: nacelle.Event{
		Kind: nacelle.KindToolResult,
		Tool: &nacelle.ToolEvent{ID: "1", Name: "run_command"},
	}})

	if printed := printedBy(cmd); !strings.Contains(printed, "run_command") {
		t.Errorf("printed = %q, want the line said before the wait for the next event", printed)
	}
}

// The question has to reach the screen when it is asked, not when it is
// answered. waitFor blocks until the model's first token, a batch is not
// finished until every command in it is, and Update sequences the print queue
// behind whatever the routed message returned — so the echo sat there until
// the reply arrived. What that looked like was the prompt emptying and nothing
// else happening, which reads as a client that swallowed the question.
func TestTheQuestionIsPrintedWithoutWaitingOnTheAnswer(t *testing.T) {
	m := sized()
	m.agent = answering(t)
	m.prompt.SetValue("a question")

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	printed := printedBy(cmd)
	defer m.run.cancel()

	if !strings.Contains(printed, "a question") {
		t.Errorf("printed = %q, want the question handed over before the run is waited on", printed)
	}
}

func TestKindTurnRendersBoundaryWithoutBreakingConversation(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.cancel = func() {}
	m.run.turnBegan = time.Now().Add(-1500 * time.Millisecond)

	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "the answer\n"})
	m.record(nacelle.Event{Kind: nacelle.KindText, Text: "the answer\n"})
	m.absorb(nacelle.Event{Kind: nacelle.KindTurn, Usage: nacelle.Usage{InputTokens: 100, OutputTokens: 50, Cost: 0.0025}})
	m.absorb(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopEnd})
	m.settle()

	saidLines := spoken(m)
	if len(saidLines) != 2 || !strings.Contains(saidLines[0], "the answer") {
		t.Fatalf("spoken = %v, want answer and turn boundary", saidLines)
	}
	if !strings.Contains(saidLines[1], "150 tokens") || !strings.Contains(saidLines[1], "$0.0025") {
		t.Errorf("turn boundary = %q, want tokens and cost", saidLines[1])
	}
	raw := m.unprinted[len(m.unprinted)-1]
	if !strings.Contains(raw, "\x1b[38;5;2") {
		t.Errorf("turn boundary raw = %q, want muted style", raw)
	}
	if len(m.conversation) != 1 || said(m.conversation[0]) != "the answer\n" {
		t.Errorf("conversation = %v, want full answer preserved", m.conversation)
	}
}

// The finished widgets of a turn — the thinking line, the turn boundary, and
// each tool line — get a blank row after them so they do not run into whatever
// comes next. Without it the boundary "3.069s · 131k tokens · $0.0011" sat
// glued to the next tool call and every tool line stuck to the one below it,
// which read as one dense block instead of separate steps.
func TestWidgetLinesAreSeparatedByBlankRows(t *testing.T) {
	m := sized()
	m.run.answer.WriteString("the answer")
	m.turn(nacelle.Event{Usage: nacelle.Usage{InputTokens: 10}})
	m.say(fromTool, "$ read_file(x)")

	said := visible(strings.Join(m.unprinted, "\n"))
	boundaryAt := strings.Index(said, "10 tokens")
	toolAt := strings.Index(said, "$ read_file(x)")
	if boundaryAt < 0 || toolAt < 0 {
		t.Fatalf("said = %q, want boundary and tool line", said)
	}
	if boundaryAt >= toolAt {
		t.Errorf("said = %q, want the turn boundary before the tool line", said)
	}
	if gap := said[boundaryAt:toolAt]; strings.Count(gap, "\n") < 2 {
		t.Errorf("said = %q, want a blank row between the turn boundary and the tool line", said)
	}
	if before := said[:boundaryAt]; strings.Count(before, "\n") < 2 {
		t.Errorf("said = %q, want a blank row between the answer and the turn boundary", said)
	}
}

// A turn boundary closes the answer it streamed under, so it must be held apart
// from that answer by a blank row — not just from the tool line that follows. A
// single answer line and a boundary gluing straight to it reads as the timing
// being part of the answer, which is what the user reported ("11.159s · 318k
// tokens · $0.0028" sitting under "Code committed.").
func TestTurnBoundaryIsSeparatedFromTheAnswerAboveIt(t *testing.T) {
	m := sized()
	m.run.answer.WriteString("Code committed")
	m.turn(nacelle.Event{Usage: nacelle.Usage{InputTokens: 10}})

	said := visible(strings.Join(m.unprinted, "\n"))
	boundaryAt := strings.Index(said, "10 tokens")
	answerAt := strings.Index(said, "Code committed")
	if boundaryAt < 0 || answerAt < 0 || answerAt >= boundaryAt {
		t.Fatalf("said = %q, want the answer before the boundary", said)
	}
	if gap := said[answerAt:boundaryAt]; strings.Count(gap, "\n") < 2 {
		t.Errorf("said = %q, want a blank row between the answer and the turn boundary", said)
	}
}

// The reader's question is a block of its own: it sits a blank row under the
// thinking trace and a blank row over whatever follows, the same breathing
// room a turn boundary gets, so it reads as the thing you scrolled up to find
// rather than a question glued to the tool call it prompted.
func TestReaderQuestionIsSeparatedFromTheTraceAroundItByBlankRows(t *testing.T) {
	m := sized()
	m.say(fromThinking, "thinking out loud")
	m.say(fromReader, "a question")
	m.say(fromTool, "$ read_file(x)")

	said := visible(strings.Join(m.unprinted, "\n"))
	thinkingAt := strings.Index(said, "thinking out loud")
	questionAt := strings.Index(said, "a question")
	toolAt := strings.Index(said, "$ read_file(x)")
	if thinkingAt < 0 || questionAt < 0 || toolAt < 0 {
		t.Fatalf("said = %q, want thinking, question, and tool line", said)
	}
	if above := said[thinkingAt:questionAt]; strings.Count(above, "\n") < 2 {
		t.Errorf("said = %q, want a blank row between the thinking trace and the question", said)
	}
	if below := said[questionAt:toolAt]; strings.Count(below, "\n") < 2 {
		t.Errorf("said = %q, want a blank row between the question and the tool line", said)
	}
}
