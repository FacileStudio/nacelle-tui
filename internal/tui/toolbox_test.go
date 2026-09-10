package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

// A run_command's output is drawn as a full-width box sharing the diff's pane,
// left border green for a call that worked, so a command's stdout and stderr
// are as visible in the transcript as the file edits around them.
func TestARunCommandOutputRendersAsABox(t *testing.T) {
	m := sized()
	m.absorb(called("c", "run_command", `{"command":"go test"}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "c", Name: "run_command", Input: `{"command":"go test"}`, Result: "ok   package\nPASS",
	}})

	said := strings.Join(m.unprinted, "\n")
	if !strings.Contains(said, "38;5;32") {
		t.Errorf("output = %q, want the success box's green border", said)
	}
	if !strings.Contains(visible(said), "ok   package") || !strings.Contains(visible(said), "PASS") {
		t.Errorf("output = %q, want the command's output in the box", visible(said))
	}
	for line := range strings.SplitSeq(visible(said), "\n") {
		if strings.Contains(line, "│") && len([]rune(line)) != 80 {
			t.Errorf("box row = %q, want a full 80-cell pane", line)
		}
	}
}

// A failed command still shows its output in a box, but the left border turns
// red so the eye lands on the call that went wrong.
func TestAFailedRunCommandRendersARedOutputBox(t *testing.T) {
	m := sized()
	m.absorb(called("f", "run_command", `{"command":"bad"}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "f", Name: "run_command", Input: `{"command":"bad"}`, Result: "boom: lost the build", Err: errors.New("exit 1"),
	}})
	m.stranded()

	said := strings.Join(m.unprinted, "\n")
	if !strings.Contains(said, "38;5;31") {
		t.Errorf("output = %q, want the failure box's red border", said)
	}
	if !strings.Contains(visible(said), "boom: lost the build") {
		t.Errorf("output = %q, want the failed command's output in a box", visible(said))
	}
}

// A finished edit draws its file diff in a box whose footer sums the change as
// "+x -y", additions counted and coloured separately from removals.
func TestAFinishedEditRendersARecapBox(t *testing.T) {
	m := sized()
	m.run.diffs = true
	m.absorb(called("e", "edit_file", `{"path":"view.go","old":"old","new":"new"}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "e", Name: "edit_file", Input: `{"path":"view.go","old":"old","new":"new"}`,
	}})

	said := visible(strings.Join(m.unprinted, "\n"))
	if !strings.Contains(said, "view.go") {
		t.Errorf("diff = %q, want the changed file named in the box", said)
	}
	if !strings.Contains(said, "+1 -1") {
		t.Errorf("diff = %q, want the +1 -1 recap", said)
	}
}

// While an edit or command is still running, its live region row is the same
// box the result will fill, wearing the tool's own colour on the left border.
func TestTheLiveBoxWearsTheToolColourWhileRunning(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.absorb(called("l", "edit_file", `{"path":"view.go"}`))

	view := m.View().Content
	if !strings.Contains(view, "38;5;35") {
		t.Errorf("view = %q, want the running edit box's magenta border", view)
	}
	if !strings.Contains(view, "│") {
		t.Errorf("view = %q, want a boxed left border while running", view)
	}
}

// A read call is not boxed while running — only edits and commands get the
// pane treatment, so the box means "this changed the tree".
func TestANonEditToolIsNotBoxedWhileRunning(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.absorb(called("r", "read_file", `{"path":"view.go"}`))

	if strings.Contains(m.View().Content, "│") {
		t.Error("a read call was boxed, want the ordinary held line")
	}
}

// A command that returns no output has nothing to box, so it stays the plain
// one-line report rather than drawing an empty pane.
func TestARunCommandWithNoOutputRendersNoBox(t *testing.T) {
	m := sized()
	m.absorb(called("x", "run_command", `{"command":"true"}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "x", Name: "run_command", Input: `{"command":"true"}`, Result: "",
	}})

	if strings.Contains(visible(strings.Join(m.unprinted, "\n")), "│") {
		t.Error("a command with no output still drew a box")
	}
}

// Streamed output from a running command fills the box live, made visible in
// the running tool's live region under the held line.
func TestAStreamingCommandFillsTheLiveBox(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.absorb(called("s", "run_command", `{"command":"make"}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolOutput, Tool: &nacelle.ToolEvent{ID: "s", Name: "run_command"}, Text: "compling...\n"})

	view := m.View().Content
	if !strings.Contains(visible(view), "compling...") {
		t.Errorf("view = %q, want the streamed line in the running box", visible(view))
	}
}

// Streamed output that filled the box wins over the result's copy of the same
// output, so a backend that streams never doubles the box when the result
// arrives carrying the whole output again.
func TestStreamedOutputIsNotDuplicatedAtResult(t *testing.T) {
	m := sized()
	m.absorb(called("d", "run_command", `{"command":"make"}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolOutput, Tool: &nacelle.ToolEvent{ID: "d", Name: "run_command"}, Text: "go build\n"})
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "d", Name: "run_command", Input: `{"command":"make"}`, Result: "go build\n",
	}})

	said := visible(strings.Join(m.unprinted, "\n"))
	if !strings.Contains(said, "go build") {
		t.Errorf("output = %q, want the streamed line in the committed box", said)
	}
	if strings.Count(said, "go build") != 1 {
		t.Errorf("output = %q, want the streamed line once, not duplicated by the result", said)
	}
}
