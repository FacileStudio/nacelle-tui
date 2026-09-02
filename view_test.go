package main

import (
	"errors"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

// called is the call event every test about tool lines starts from.
func called(id, name, input string) nacelle.Event {
	return nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: id, Name: name, Input: input}}
}

// A call line carries its own duration, so it cannot be said until the result
// arrives — a printed line belongs to the terminal and is never rewritten.
// Nothing is lost meanwhile: the status line names the tool that is running.
func TestACallSaysNothingUntilItsResultArrives(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.absorb(called("1", "read_file", `{"path":"view.go"}`))

	if lines := spoken(m); len(lines) != 0 {
		t.Errorf("transcript = %v, want the line held until the result", lines)
	}
	if status := visible(m.status()); !strings.Contains(status, "running read_file") {
		t.Errorf("status = %q, want the held tool named while it runs", status)
	}
}

// A failure is the one thing a reader must not scroll past, and the error has
// nowhere to fit on a line already ending in a duration.
func TestAFailedToolKeepsItsCallLineAndItsError(t *testing.T) {
	m := sized()
	m.absorb(called("1", "run_command", `{"command":"go build ./..."}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID:       "1",
		Name:     "run_command",
		Err:      errors.New("exit status 2"),
		Duration: 12 * time.Millisecond,
	}})
	m.stranded()

	lines := spoken(m)
	if len(lines) != 1 {
		t.Fatalf("transcript = %v, want the call and the failure in one entry", lines)
	}
	if !strings.Contains(lines[0], "run_command(go build ./...)") {
		t.Errorf("call line = %q, want the call named without a duration", lines[0])
	}
	if !strings.Contains(lines[0], "exit status 2") || !strings.Contains(lines[0], "12ms") {
		t.Errorf("failure line = %q, want the error and how long it took", lines[0])
	}
}

// Identical consecutive failures collapse into one line with a count. The
// first prints normally; the second extends the collapse rather than
// printing another identical block.
func TestIdenticalFailuresCollapse(t *testing.T) {
	m := sized()
	// Two identical calls, both failing with the same error.
	m.absorb(called("1", "run_command", `{"command":"go build ./..."}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "1", Name: "run_command",
		Err:      errors.New("exit status 2"),
		Duration: 12 * time.Millisecond,
	}})
	m.absorb(called("2", "run_command", `{"command":"go build ./..."}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "2", Name: "run_command",
		Err:      errors.New("exit status 2"),
		Duration: 15 * time.Millisecond,
	}})
	m.stranded()

	lines := spoken(m)
	if len(lines) != 1 {
		t.Fatalf("transcript = %v, want one collapsed failure entry", lines)
	}
	if !strings.Contains(lines[0], "run_command(go build ./...)") {
		t.Errorf("call line = %q, want the call named", lines[0])
	}
	if !strings.Contains(lines[0], "2 times") {
		t.Errorf("failure = %q, want the count showing the two collapses", lines[0])
	}
}

// Different failures are not collapsed — each keeps its own line.
func TestDifferentFailuresAreNotCollapsed(t *testing.T) {
	m := sized()
	m.absorb(called("1", "run_command", `{"command":"go build ./..."}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "1", Name: "run_command",
		Err:      errors.New("exit status 2"),
		Duration: 12 * time.Millisecond,
	}})
	m.absorb(called("2", "run_command", `{"command":"go test ./..."}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "2", Name: "run_command",
		Err:      errors.New("test failure"),
		Duration: 20 * time.Millisecond,
	}})
	m.stranded()

	lines := spoken(m)
	if len(lines) != 2 {
		t.Fatalf("transcript = %v, want two separate failure entries", lines)
	}
}


// A discarded call belongs to an attempt that was superseded and never ran, so
// announcing it would be the transcript describing work nobody did.
func TestADiscardedCallDropsItsHeldLine(t *testing.T) {
	m := sized()
	m.absorb(called("1", "read_file", `{"path":"view.go"}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "1", Name: "read_file", Discarded: true,
	}})

	if lines := spoken(m); len(lines) != 0 {
		t.Errorf("transcript = %v, want the superseded call dropped rather than printed", lines)
	}
	if n := m.running(); n != 0 {
		t.Errorf("running = %v, want the discarded call forgotten", n)
	}
}

// A run capped at its iteration limit reports tools it never runs, and an
// abandoned one never reaches the results of the tools it was mid-way through.
// A held line those never answer is a line nobody would ever see.
func TestARunThatEndsMidToolStillSaysWhatItAskedFor(t *testing.T) {
	m := sized()
	m.run.cancel = func() {}
	m.run.busy = true
	m.absorb(called("a", "read_file", `{"path":"view.go"}`))
	m.absorb(called("b", "run_command", `{"command":"go test ./..."}`))

	m.settle()

	lines := spoken(m)
	if len(lines) != 2 {
		t.Fatalf("transcript = %v, want both un-resulted calls said", lines)
	}
	if !strings.Contains(lines[0], "read_file(view.go)") || !strings.Contains(lines[1], "run_command(go test ./...)") {
		t.Errorf("transcript = %v, want the calls in the order they were asked for", lines)
	}
	if strings.Contains(strings.Join(lines, "\n"), " · ") {
		t.Errorf("transcript = %v, want no duration on a call that never returned", lines)
	}
}

// Everything finished is printed to the terminal, so the status line is drawn
// directly beneath the last line of an answer that is no longer this client's
// to redraw. Without a row between them the token count reads as part of the
// sentence above it.
func TestABlankRowSeparatesTheAnswerFromTheStatusLine(t *testing.T) {
	m := sized()

	lines := strings.Split(visible(m.View().Content), "\n")
	status := -1
	for i, line := range lines {
		if strings.Contains(line, "ready") {
			status = i
			break
		}
	}
	if status < 1 {
		t.Fatalf("no status line found in\n%s", strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(lines[status-1]) != "" {
		t.Errorf("row above the status line = %q, want it blank", lines[status-1])
	}
}

// testedUltraviolet is the github.com/charmbracelet/ultraviolet commit that
// charm.land/bubbletea/v2 v2.0.9 requires, and the newest one this client is
// known to draw correctly on.
//
// A later commit moved curbuf.Resize ahead of the clear pass in Render, so
// TerminalRenderer.move clamps its remembered cursor row against the frame
// that is arriving instead of the one on screen. In inline mode that row is
// the only record of where the cursor physically is, so every shrink climbed
// too few rows and redrew the frame below the rows it had failed to erase:
// closing the / dropdown left its list painted with a stale status line above
// it, and deleting an alt+enter line walked the frame down the pane.
//
// Nothing this client does can detect that from inside, so the version is
// pinned instead. If bubbletea moves to a newer ultraviolet, run the shrink by
// hand before following it: type "/", press backspace, and look for leftover
// dropdown rows.
const testedUltraviolet = "v0.0.0-20260703014108-f5a850f9c2b7"

// The renderer that draws this client's frame is two modules down and cannot
// be reached from a test, so this asserts the version rather than the drawing.
//
// It fails rather than skips when the build info is missing: a tripwire that
// can quietly decide not to fire is not one. Build info is present under the
// workspace, under GOWORK=off, and under -race, which is every way this runs.
func TestTheInlineRendererIsTheCommitBubbleteaWasTestedAgainst(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("no build info in this binary, so the pin below went unchecked")
	}
	for _, dep := range info.Deps {
		if dep.Path != "github.com/charmbracelet/ultraviolet" {
			continue
		}
		if dep.Version != testedUltraviolet {
			t.Errorf("ultraviolet = %s, want %s: re-verify the shrink before moving it",
				dep.Version, testedUltraviolet)
		}
		return
	}
	t.Fatal("ultraviolet is not in the build graph")
}

// A run that ends mid-tool says two things, and the order they land in is the
// order they happened in. The held call line is the tool the model announced,
// and closeTurn is what flushes the sentence announcing it — saying the line
// first put the tool above the text that asked for it. consume gets this right
// on the ordinary path by recording before it absorbs; settle is the path that
// does not go through consume at all.
func TestAnEndedRunSaysTheAnswerBeforeTheToolItWasStillHolding(t *testing.T) {
	m := newModel(nil, "banner", nil)
	m.width = 80
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "reading the file now"})
	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{
		ID: "1", Name: "read_file", Input: `{"path":"view.go"}`,
	}})
	m.unprinted = nil

	m.settle()

	said := strings.Join(m.unprinted, "\n")
	answer := strings.Index(said, "reading the file")
	call := strings.Index(said, "read_file(")
	if answer < 0 || call < 0 {
		t.Fatalf("both the answer and the held call line must survive the run ending: %q", said)
	}
	if answer > call {
		t.Errorf("the tool line was said above the sentence announcing it")
	}
}
