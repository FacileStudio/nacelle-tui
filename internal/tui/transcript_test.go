package tui

import (
	"fmt"
	"strings"
	"testing"
)

// Nothing is printed until Update drains the queue, because printing is a Cmd
// and say has callers that cannot return one.
func TestSayingSomethingQueuesItForTheTerminal(t *testing.T) {
	m := sized()
	m.say(fromReader, "a question")

	if said := spoken(m); len(said) != 1 || !strings.Contains(said[0], "a question") {
		t.Errorf("said = %v, want the one line waiting to be printed", said)
	}
}

// The reader's own question is rendered with a muted background and a bold
// pipe prefix so it stands out from the model's markdown answer without
// reading like a chat bubble.
func TestReaderQuestionIsPaintedWithMutedBackgroundAndBoldPipePrefix(t *testing.T) {
	m := sized()
	m.say(fromReader, "a question")

	said := spoken(m)
	if len(said) != 1 {
		t.Fatalf("spoken = %v, want 1 line", said)
	}
	line := said[0]
	if !strings.Contains(line, "a question") {
		t.Errorf("spoken line = %q, want the question text", line)
	}
	if !strings.Contains(line, "| ") {
		t.Errorf("spoken line = %q, want bold pipe prefix", line)
	}
}

// One Println for the batch, not one per line: tea.Batch promises nothing
// about the order its commands run in, and a transcript out of order is not a
// transcript. The body is read back through fmt because the message type is
// the library's own and unexported.
func TestEverythingSaidGoesOutAsOneMessageInOrder(t *testing.T) {
	m := sized()
	m.say(fromReader, "asked first")
	m.say(fromModel, "answered second")

	cmd := m.prints()
	if cmd == nil {
		t.Fatal("nothing was printed, want the queued lines handed to the terminal")
	}

	body := visible(fmt.Sprint(cmd()))
	first, second := strings.Index(body, "asked first"), strings.Index(body, "answered second")
	if first < 0 || second < 0 {
		t.Fatalf("printed = %q, want both lines in the one message", body)
	}
	if first > second {
		t.Errorf("printed = %q, want the question before the answer", body)
	}
}

// Draining has to empty the queue, or every message reprints everything said
// before it.
func TestPrintingForgetsWhatItHandedOver(t *testing.T) {
	m := sized()
	m.say(fromReader, "a question")
	m.prints()

	if len(m.unprinted) != 0 {
		t.Errorf("unprinted = %v, want it emptied once handed to the terminal", m.unprinted)
	}
	if m.prints() != nil {
		t.Error("a second drain produced a command, want nothing left to print")
	}
}

// The live region is repainted in place on every delta, so it has to fit on
// the screen. Content taller than the terminal cannot be redrawn where it
// streaming only holds partial lines — every completed line is committed to
// scrollback immediately by commitParagraphs.
func TestStreamingHoldsOnlyPartialLines(t *testing.T) {
	m := sized()
	m.run.answer.WriteString(strings.Repeat("a line of streamed answer\n", 40))
	m.commitParagraphs()

	if got := m.run.answer.Len(); got > 0 {
		t.Errorf("answer buffer = %d bytes after 40 complete lines, want 0", got)
	}
}

// flush returns the full answer even though lines were already committed to
// scrollback during streaming — fullAnswer accumulates everything.
func TestFlushReturnsTheFullAnswer(t *testing.T) {
	m := sized()
	m.liveRows = 2
	m.run.answer.WriteString("the opening line\nand many more\nand the last one")
	m.run.fullAnswer.WriteString("the opening line\nand many more\nand the last one")

	m.flush()

	if said := strings.Join(spoken(m), "\n"); !strings.Contains(said, "the opening line") {
		t.Errorf("said = %q, want the start of the answer printed, not only the visible tail", said)
	}
}

// TestCommitParagraphsTracksCommittedBytes ensures commitParagraphs advances
// the committed length so flush does not re-print what was already said.
// Note: commitParagraphs commits up to and including the LAST newline in the
// buffer, which matches the existing behavior.
func TestCommitParagraphsTracksCommittedBytes(t *testing.T) {
	m := sized()
	m.run.answer.WriteString("first line\nsecond line\npartial")
	m.run.fullAnswer.WriteString("first line\nsecond line\npartial")

	m.commitParagraphs()

	want := "first line\nsecond line\n"
	if m.run.committedLen != len(want) {
		t.Errorf("committedLen = %d, want %d after committing up to last newline", m.run.committedLen, len(want))
	}

	answer := m.flush()
	if answer != "first line\nsecond line\npartial" {
		t.Errorf("flush() = %q, want full answer", answer)
	}
}

// TestFlushDoesNotDuplicateCommittedLines ensures that lines already printed
// via commitParagraphs are not re-printed by flush.
func TestFlushDoesNotDuplicateCommittedLines(t *testing.T) {
	m := sized()
	m.run.answer.WriteString("line one\nline two\npartial")
	m.run.fullAnswer.WriteString("line one\nline two\npartial")

	m.commitParagraphs()

	committed := strings.Join(spoken(m), "\n")

	answer := m.flush()
	if answer != "line one\nline two\npartial" {
		t.Errorf("flush() returned %q, want full answer", answer)
	}
	if !strings.Contains(committed, "line one") {
		t.Errorf("commitParagraphs did not print 'line one', want it in scrollback")
	}
	if !strings.Contains(committed, "line two") {
		t.Errorf("commitParagraphs did not print 'line two', want it in scrollback")
	}

	allSpoken := strings.Join(spoken(m), "\n")
	if strings.Count(allSpoken, "line one") != 1 {
		t.Errorf("line one printed %d times, want 1", strings.Count(allSpoken, "line one"))
	}
	if strings.Count(allSpoken, "line two") != 1 {
		t.Errorf("line two printed %d times, want 1", strings.Count(allSpoken, "line two"))
	}
	if !strings.Contains(allSpoken, "partial") {
		t.Errorf("flush did not print 'partial' to scrollback")
	}
}
