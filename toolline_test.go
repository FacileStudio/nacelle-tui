package main

import (
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle"
)

// A call line is only worth reading if it names the thing the tool acted on,
// and the input it has to find that in is whatever schema the tool happens to
// have — including tools this client has never heard of, reached over MCP.
func TestThePrimaryArgumentIsPickedFromWhateverShapeArrives(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"a known key", `{"path":"view.go"}`, "view.go"},
		{"the first known key wins", `{"query":"tools","path":"view.go"}`, "view.go"},
		{"a command", `{"command":"go test ./...","timeout":60}`, "go test ./..."},
		{"the noise around it is dropped", `{"file_path":"run.go","offset":0,"limit":2000}`, "run.go"},
		{"the only key, whatever it is called", `{"expression":"1+1"}`, "1+1"},
		{"nothing to pick between unknown keys", `{"left":1,"right":2}`, ""},
		{"a value that is not a string keeps its json", `{"lines":[1,2,3]}`, "[1,2,3]"},
		{"a number", `{"limit":40}`, "40"},
		{"whitespace is collapsed onto the one line", "{\"command\":\"a\\n  b\"}", "a b"},
		{"an empty object", `{}`, ""},
		{"no input at all", ``, ""},
		{"input that is not an object", `"just a string"`, ""},
		{"input that is not json", `not json at all`, ""},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := primaryArg(test.input); got != test.want {
				t.Errorf("primaryArg(%s) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

// Raw JSON in the transcript is what this replaced, so the shapes that have no
// argument worth naming have to degrade to the short honest thing rather than
// falling back to the blob.
func TestACallWithNothingWorthNamingIsJustTheToolName(t *testing.T) {
	if got := toolLine("read_file", `not json`, 80); got != "☰ read_file()" {
		t.Errorf("line = %q, want the tool name and empty parentheses", got)
	}
	if got := toolLine("list_tools", ``, 80); got != "• list_tools()" {
		t.Errorf("line = %q, want the tool name and empty parentheses", got)
	}
}

// The line is printed into the terminal's own scrollback and can never be
// redrawn, so one that does not fit is one that wraps for good.
func TestALongArgumentIsCutToFitTheLine(t *testing.T) {
	long := `{"path":"` + strings.Repeat("deep/", 40) + `file.go"}`
	line := toolLine("read_file", long, 40)

	if width := lipgloss.Width(line); width > 40-durationRoom {
		t.Errorf("line is %d cells wide: %q, want it inside the width less the room kept for the duration", width, line)
	}
	if !strings.HasPrefix(line, "☰ read_file(deep/") || !strings.HasSuffix(line, "…)") {
		t.Errorf("line = %q, want the argument cut with an ellipsis", line)
	}
}

// A window too narrow to hold anything is not a reason to wrap; the name alone
// is what survives.
func TestANarrowWindowKeepsTheNameAndDropsTheArgument(t *testing.T) {
	if got := toolLine("run_command", `{"command":"go build ./..."}`, 12); got != "$ run_command()" {
		t.Errorf("line = %q, want the argument dropped rather than wrapped", got)
	}
}

// Rounding a fast tool to zero reads as a broken clock rather than as a fast
// tool.
func TestASubMillisecondCallIsFlooredRatherThanRoundedToZero(t *testing.T) {
	if got := took(400_000); got != "1ms" {
		t.Errorf("took(400µs) = %q, want it floored to a millisecond", got)
	}
}

// The one input where naming an argument would be a lie. encoding/json keeps
// the last value of a repeated key without saying so, and the gate that
// refuses such a call outright is only on when -approve-tools asked for it —
// so the transcript is what has to say the line cannot be trusted.
func TestARepeatedKeyIsSaidRatherThanSummarised(t *testing.T) {
	got := toolLine("run_command", `{"command":"ls","command":"rm -rf /"}`, 80)
	if got != "$ run_command(input has a duplicate key)" {
		t.Errorf("line = %q, want the ambiguity reported instead of one of the two values", got)
	}
}

// TestADiscardedCallIsNotCountedAsWorkDone pins the one thing about the
// session tally that is not obvious: a call belonging to a superseded attempt
// never ran, so counting it would have the closing recap claim work nobody
// did. Every other call, failures included, is counted exactly once.
func TestADiscardedCallIsNotCountedAsWorkDone(t *testing.T) {
	m := bareBanner()

	m.finished(&nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"x"}`})
	m.finished(&nacelle.ToolEvent{ID: "b", Name: "run_command", Input: `{"command":"x"}`, Err: errors.New("nope")})
	m.finished(&nacelle.ToolEvent{ID: "c", Name: "read_file", Input: `{"path":"y"}`, Discarded: true})

	if m.tools != 2 {
		t.Errorf("counted %d tools, want 2 (the discarded call must not count)", m.tools)
	}
	if m.failed != 1 {
		t.Errorf("counted %d failures, want 1", m.failed)
	}
}

// TestGroupOnlyPrintsOnce verifies that a kind-based batch renders as a single
// line, not N copies of the same group. finished() must only emit when the
// last call in the group lands.
func TestGroupOnlyPrintsOnce(t *testing.T) {
	m := sized()
	m.groupTools = true

	// Begin a group of three read_file calls.
	m.run.beginTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "c", Name: "read_file", Input: `{"path":"inflight.go"}`}, true)

	if m.run.groups[0].count != 3 {
		t.Fatalf("group count = %d, want 3", m.run.groups[0].count)
	}

	// Helper to check what's in the transcript (excluding banner).
	said := func() []string { return spoken(m) }

	// Finish each call one by one. The group line must not appear until the
	// last one lands. Each call goes through the real flow: finishTool then finished.
	m.run.finishTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`})
	m.finished(&nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`})
	if lines := said(); len(lines) != 0 {
		t.Errorf("after 1st finish: %v, want empty transcript", lines)
	}

	m.run.finishTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`})
	m.finished(&nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`})
	if lines := said(); len(lines) != 0 {
		t.Errorf("after 2nd finish: %v, want empty transcript", lines)
	}

	m.run.finishTool(nacelle.ToolEvent{ID: "c", Name: "read_file", Input: `{"path":"inflight.go"}`})
	m.finished(&nacelle.ToolEvent{ID: "c", Name: "read_file", Input: `{"path":"inflight.go"}`})
	lines := said()
	if len(lines) != 1 {
		t.Fatalf("after 3rd finish: %d lines %v, want exactly 1", len(lines), lines)
	}
	if !strings.Contains(lines[0], "☰ 3 reads") {
		t.Errorf("group line = %q, want '☰ 3 reads'", lines[0])
	}
}

// TestAGroupCallsTrackArgumentsNotNames verifies that kind-based grouping
// records the primary argument of each call in the group, not the tool name.
// A batch of four read_file calls should show as "⏺ 4 reads · file_a · file_b · …"
// rather than repeating "read_file" four times.
func TestAGroupCallsTrackArgumentsNotNames(t *testing.T) {
	m := bareBanner()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "1", Name: "read_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "2", Name: "read_file", Input: `{"path":"run.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "3", Name: "read_file", Input: `{"path":"inflight.go"}`}, true)

	// The group should have collected the three arguments, not the name repeated
	g := m.run.groups[0]
	if g.count != 3 {
		t.Errorf("group count = %d, want 3", g.count)
	}
	if len(g.callNames) != 3 {
		t.Fatalf("callNames = %v, want 3 entries", g.callNames)
	}
	want := []string{"view.go", "run.go", "inflight.go"}
	for i, w := range want {
		if g.callNames[i] != w {
			t.Errorf("callNames[%d] = %q, want %q", i, g.callNames[i], w)
		}
	}
}

// TestGroupLineShowsArguments verifies the rendered group line names the
// arguments, not the tool name repeated.
func TestGroupLineShowsArguments(t *testing.T) {
	m := bareBanner()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "1", Name: "run_command", Input: `{"command":"ls docs"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "2", Name: "run_command", Input: `{"command":"git status"}`}, true)

	line := m.run.groups[0].groupLine(m.width)
	if !strings.Contains(line, "ls docs") {
		t.Errorf("group line = %q, want it to contain 'ls docs'", line)
	}
	if !strings.Contains(line, "git status") {
		t.Errorf("group line = %q, want it to contain 'git status'", line)
	}
	if strings.Count(line, "run_command") > 1 {
		t.Errorf("group line = %q, want the tool name mentioned at most once", line)
	}
}
