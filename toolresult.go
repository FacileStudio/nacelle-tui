package main

import (
	"fmt"
	"strings"
	"time"

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
	m.session.tool(tool.Name, tool.Duration)

	if tool.Err != nil {
		m.failed++
	}

	if !m.canPrintTool(tool.ID, held) {
		return
	}

	if held {
		if g := m.run.findGroup(tool.ID); g != nil && g.count > 1 {
			dur := took(g.duration())
			if g.failed {
				m.printGroupFailure(line, tool.Name, g.errors, dur)
			} else {
				m.say(fromTool, colorGlyph(line, "32", toolRestore(tool.Name))+" · "+dur)
			}
			for _, id := range g.callIDs {
				m.finishEdit(id)
			}
			return
		}
	}

	if tool.Err != nil {
		m.trackFailure(line, tool)
		return
	}

	m.flushFailures()
	m.say(fromTool, colorGlyph(line, "32", toolRestore(tool.Name))+" · "+took(tool.Duration))
	m.finishEdit(tool.ID)
}

func (m *model) printGroupFailure(line, name string, errs []toolError, dur string) {
	styled := colorGlyph(line, "31", toolRestore(name)) + " · " + dur
	toolStr := m.paint(fromTool, styled)
	m.session.line(fromTool, styled)
	if len(errs) == 0 {
		m.unprinted = append(m.unprinted, toolStr)
		return
	}
	type collapse struct {
		toolError
		count int
	}
	var collapsed []collapse
	for _, e := range errs {
		if n := len(collapsed); n > 0 && collapsed[n-1].name == e.name && collapsed[n-1].err == e.err {
			collapsed[n-1].count++
			collapsed[n-1].duration = e.duration
		} else {
			collapsed = append(collapsed, collapse{toolError: e, count: 1})
		}
	}
	lines := []string{toolStr}
	for _, c := range collapsed {
		text := fmt.Sprintf("%s failed after %s: %s", c.name, took(c.duration), c.err)
		if c.count > 1 {
			text = fmt.Sprintf("%s failed %d times · last: %s: %s", c.name, c.count, took(c.duration), c.err)
		}
		lines = append(lines, m.paint(fromResult, text))
		m.session.line(fromResult, text)
	}
	m.unprinted = append(m.unprinted, strings.Join(lines, "\n"))
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
