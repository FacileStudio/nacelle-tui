package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FacileStudio/nacelle"
)

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
		m.trackFailure(line, tool)
	}

	if !m.canPrintTool(tool.ID, held) {
		return
	}

	if held && m.run.isGroup(tool.ID) && m.run.isGroupFailed(tool.ID) {
		m.flushFailures()
		return
	}

	if tool.Err != nil {
		return
	}

	m.flushFailures()
	m.say(fromTool, colorGlyph(line, "32", toolRestore(tool.Name))+" · "+took(tool.Duration))
	m.session.tool(tool.Name, tool.Duration)
	m.finishEdit(tool.ID)
}

func (m *model) finishEdit(id string) {
	change, edited := m.run.edits[id]
	if !edited {
		return
	}
	delete(m.run.edits, id)
	if change.after == "" && change.before != "" {
		change.after = priorContents(m.run.root, change.path)
	}
	if diff := renderDiff(change, m.width, m.theme.muted); diff != "" {
		m.say(fromDiff, diff)
	}
}

// canPrintTool returns false when a grouped tool result has not yet had its
// last call — printing the group line here would duplicate it on every call
// in the batch. Non-grouped tools always return true.
func (m *model) canPrintTool(id string, held bool) bool {
	if !held {
		return true
	}
	return m.run.isGroupComplete(id)
}

// trackFailure increments the failure counter and either extends an existing
// collapse batch or starts a new one. The collapse matches on (name, err), so
// identical failures render as "name · N times · duration" rather than one
// line per repeated error.
func (m *model) trackFailure(line string, tool *nacelle.ToolEvent) {
	errText := tool.Err.Error()
	if m.run.failures.name == tool.Name && m.run.failures.err == errText {
		m.run.failures.count++
		m.run.failures.duration = tool.Duration
		return
	}
	m.flushFailures()
	m.run.failures = failureCollapse{
		toolLine: line,
		name:     tool.Name,
		err:      errText,
		duration: tool.Duration,
		count:    1,
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
