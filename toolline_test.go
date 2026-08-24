package main

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
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
	if got := toolLine("read_file", `not json`, 80); got != "read_file()" {
		t.Errorf("line = %q, want the tool name and empty parentheses", got)
	}
	if got := toolLine("list_tools", ``, 80); got != "list_tools()" {
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
	if !strings.HasPrefix(line, "read_file(deep/") || !strings.HasSuffix(line, "…)") {
		t.Errorf("line = %q, want the argument cut with an ellipsis", line)
	}
}

// A window too narrow to hold anything is not a reason to wrap; the name alone
// is what survives.
func TestANarrowWindowKeepsTheNameAndDropsTheArgument(t *testing.T) {
	if got := toolLine("run_command", `{"command":"go build ./..."}`, 12); got != "run_command()" {
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
	if got != "run_command(input has a duplicate key)" {
		t.Errorf("line = %q, want the ambiguity reported instead of one of the two values", got)
	}
}
