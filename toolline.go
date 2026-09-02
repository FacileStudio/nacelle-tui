package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle"
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
	// Pad 1-cell-wide glyphs with an extra space so every tool line's name
	// starts at the same column — ✎ and $ at the same visual position.
	if lipgloss.Width(glyph) == 1 {
		glyph += " "
	}
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

// finished is a tool call reaching its end, which is where its line is finally
// said.
//
// Holding the line until here is the whole point. A tool that worked is one
// line carrying its own duration, because the two-line version — the call,
// then `read_file done in 12ms` under it — doubled the height of every
// transcript to report that nothing had gone wrong. A printed line belongs to
// the terminal and can never be rewritten (see say), so the only way to fold
// the duration in is to say the line once, at the moment both halves are
// known. Nothing is hidden while it waits: the status line names the tool that
// is running for exactly as long as the line is held.
//
// A failure stays two lines and stays loud. It is the thing a reader must not
// scroll past, the model is about to be told the same thing, and the error
// itself has nowhere to fit on a line already ending in a duration.
//
// A discarded call never ran — it belongs to an attempt that was superseded,
// which is why conversation.go forgets it rather than answering it — so its
// held line is dropped rather than printed. Announcing a call the model no
// longer made is the transcript describing work nobody did.
//
// The session tally is kept here for that same reason: this is the one point a
// call is known to have both run and ended, and it sits past the discard
// check, so a superseded call is never counted as work. Nothing downstream
// could count them instead — a printed line belongs to the terminal and is
// forgotten the moment it is said.
//
// The held line lives on the run's groups rather than in a map keyed by call
// id. The group is what holds it: grouping folds a call into its row the
// moment it arrives, so a result has nowhere else to look, and a result that
// arrives for a call the model did not name still lands on the one unfinished
// row there is.
func (m *model) finished(tool *nacelle.ToolEvent) {
	if tool.Discarded {
		return
	}
	line, held := m.run.heldLine(tool.ID, m.width)
	if !held {
		line = toolLine(tool.Name, tool.Input, m.width)
	}

	m.tools++

	if tool.Err != nil {
		m.failed++

		errText := tool.Err.Error()
		if m.run.failures.name == tool.Name && m.run.failures.err == errText {
			m.run.failures.count++
			m.run.failures.duration = tool.Duration
			return
		}

		// Different failure — flush the previous batch first.
		m.flushFailures()
		m.run.failures = failureCollapse{
			toolLine: line,
			name:     tool.Name,
			err:      errText,
			duration: tool.Duration,
			count:    1,
		}
		return
	}

	// Success — flush any pending failures first.
	m.flushFailures()

	m.say(fromTool, colorGlyph(line, "32", toolRestore(tool.Name))+" · "+took(tool.Duration))
	m.session.tool(tool.Name, tool.Duration)

	if change, edited := m.run.edits[tool.ID]; edited {
		delete(m.run.edits, tool.ID)
		// run_command edits only capture the before content at call time;
		// read the file now that the command has finished to get the after.
		if change.after == "" && change.before != "" {
			change.after = priorContents(m.run.root, change.path)
		}
		if diff := renderDiff(change, m.width, m.theme.muted); diff != "" {
			m.say(fromDiff, diff)
		}
	}
}

// flushFailures renders any accumulated identical failures to the transcript
// and the session log, then resets the collapse tracker. Call it wherever a
// run ends (stranded, settle) and whenever a non-matching tool arrives.
func (m *model) flushFailures() {
	if m.run.failures.count == 0 {
		return
	}

	toolStr := m.paint(fromTool, colorGlyph(m.run.failures.toolLine, "31", toolRestore(m.run.failures.name)))
	var errLine string
	if m.run.failures.count == 1 {
		errLine = fmt.Sprintf("%s failed after %s: %s", m.run.failures.name, took(m.run.failures.duration), m.run.failures.err)
	} else {
		errLine = fmt.Sprintf("%s failed %d times · last: %s: %s", m.run.failures.name, m.run.failures.count, took(m.run.failures.duration), m.run.failures.err)
	}
	resultStr := m.paint(fromResult, errLine)
	m.unprinted = append(m.unprinted, toolStr+"\n"+resultStr)
	m.session.line(fromTool, colorGlyph(m.run.failures.toolLine, "31", toolRestore(m.run.failures.name)))
	m.session.line(fromResult, errLine)
	m.run.failures = failureCollapse{}
}

// stranded says the lines still being held for calls that never answered, and
// forgets them.
//
// Every held line is a line that has not been printed yet, so a run ending with
// calls in flight would swallow them outright — and those are not a rare shape.
// A run capped at its iteration limit reports the tools it stopped short of and
// runs none of them, and one the reader abandoned mid-tool never reaches the
// result at all. They are said without a duration, which is the honest report:
// the call was made, and how long it took is not something this client knows.
//
// The order is the groups' own order, which is the order the calls arrived in.
func (m *model) stranded() {
	m.flushFailures()
	for _, g := range m.run.groups {
		if !g.end.IsZero() {
			continue
		}
		m.say(fromTool, g.groupLine(m.width))
	}
	m.run.clearGroups()
	m.run.edits = map[string]editChange{}
}

// failed is what a tool that fell over reads as. It is reported rather than
// hidden, because the model is about to be told the same thing and the person
// watching should see what it sees.
func failed(tool *nacelle.ToolEvent) string {
	return fmt.Sprintf("%s failed after %s: %v", tool.Name, took(tool.Duration), tool.Err)
}

// took is how long a call took, at the resolution a reader cares about.
//
// A sub-millisecond call is floored to 1ms rather than rounded to zero: `· 0s`
// on a tool that plainly did something reads as a broken clock, and nobody is
// comparing 300µs against 700µs in a scrollback.
func took(spent time.Duration) string {
	return max(spent.Round(time.Millisecond), time.Millisecond).String()
}

// colorGlyph wraps the first visible rune of line (the icon) in an ANSI colour
// so only the icon changes while the rest of the line stays in restoreColour.
// The color and restoreColour are ANSI colour indices: "31" for red, "32" for
// green, "34" for the default blue, etc.
func colorGlyph(line, color, restoreColour string) string {
	for i := 0; i < len(line); i++ {
		if line[i] == '\x1b' {
			end := strings.IndexByte(line[i:], 'm')
			if end < 0 {
				break
			}
			i += end
			continue
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		if r == utf8.RuneError {
			break
		}
		before := line[:i]
		glyph := line[i : i+size]
		rest := line[i+size:]
		return before + "\x1b[" + color + "m" + glyph + "\x1b[" + restoreColour + "m" + rest
	}
	return line
}
