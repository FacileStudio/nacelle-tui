package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"charm.land/lipgloss/v2"
)

// primaryKeys is the argument a tool call is named by, in the order it is
// looked for.
//
// The alternative was a table of tool name to argument name, and it is the
// wrong shape twice over: this client does not know which tools an agent was
// built with, and half of them arrive over MCP under a server's own names. The
// keys are what the schemas agree on instead — a call carrying a path is a
// call about that path, whatever the tool reading it is called — so a tool
// nobody here has heard of still reads as `whatever(the file it touched)`.
var primaryKeys = []string{"path", "file_path", "file", "command", "pattern", "query", "url", "name"}

// durationRoom is the cells kept back from the argument for the ` · 1.234s`
// that folds in when the call finishes.
//
// The line is truncated when the call is made and the duration is not known
// until it returns, so the room for it has to be reserved rather than
// measured. Reserving too much costs a few characters of a path nobody was
// reading anyway; reserving none wraps the line, and a wrapped line in the
// terminal's scrollback can never be redrawn.
const durationRoom = 10

// toolLine is a call as the one line it reads as: the tool's name, and the one
// argument that says which thing it acted on.
//
// What this replaces printed the raw JSON input, which is how a transcript
// turns into a wall of `{"path":"view.go","offset":0,"limit":2000}` — the
// interesting eight characters of it buried in the punctuation of a wire
// format. One argument is what a reader is actually scanning for.
//
// An input that is not an object, or that has nothing worth naming, degrades to
// `name()` rather than to a guess. Empty parentheses read as "this call had no
// argument worth showing", which is true and short; the raw blob read as noise.
func toolLine(name, input string, width int) string {
	room := width - lipgloss.Width(name) - durationRoom - len("• ()")
	glyph := toolGlyph(name)
	return glyph + " " + name + "(" + truncate(unstyled(primaryArg(input)), room) + ")"
}

// primaryArg is the argument a call is named by, rendered for a single line.
//
// The single-key fallback is what covers the tools nobody agreed on: a call
// with exactly one argument is unambiguously about that argument, whatever it
// is called. Two or more unrecognised keys is where guessing stops, because
// picking one of them at random is worse than picking none.
//
// It reads the input through strictObject rather than through encoding/json
// directly, and says so out loud when that refuses. A repeated key is the one
// input where naming an argument at all would be a lie: encoding/json keeps
// the last value silently, so a line reading run_command(ls) can stand over a
// call that ran something else entirely. The approval gate denies such a call
// outright, but the gate is off unless -approve-tools asked for it — the tool
// runs, and the transcript is the only thing telling anybody what ran.
func primaryArg(input string) string {
	fields, err := strictObject([]byte(input))
	if errors.Is(err, errDuplicateKey) {
		return "input has a duplicate key"
	}
	if err != nil || len(fields) == 0 {
		return ""
	}

	for _, key := range primaryKeys {
		if raw, ok := fields[key]; ok {
			return oneLine(raw)
		}
	}
	if len(fields) == 1 {
		for _, raw := range fields {
			return oneLine(raw)
		}
	}
	return ""
}

// oneLine is one JSON value as text that fits on a transcript line.
//
// A string is unquoted, because `Read("view.go")` spends two cells on saying
// the wire format was JSON. Anything else keeps its JSON, compacted: a number
// or a list has no plainer form, and the model's own whitespace in a nested
// object would otherwise put newlines into a line that is about to be printed
// into scrollback and never redrawn.
//
// Whitespace is collapsed for that same reason. A heredoc handed to
// run_command is a perfectly ordinary argument and a perfectly terrible line.
func oneLine(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.Join(strings.Fields(text), " ")
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return ""
	}
	return strings.Join(strings.Fields(compact.String()), " ")
}
