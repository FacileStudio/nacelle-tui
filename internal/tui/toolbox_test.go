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