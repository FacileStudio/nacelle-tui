package main

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

// verbose is where the duration stops being worth a decimal. Under it a tenth
// of a second is the difference between a model that answered and one that
// stopped to think, which is the whole reason the line reports a number at
// all; over it the tenth is noise on a figure the reader is only ever going
// to read as "a while", and it costs two columns of a line that exists to be
// quiet.
const verbose = 10 * time.Second

// thoughts is what the client keeps about reasoning outside the run that
// produced it: when this turn's thinking started, the last turn's text in
// full, and whether the reader has asked to see it.
//
// It is embedded in model rather than named, the same reason commandState and
// screen are: every field still reads as m.retained or m.expanded, and the
// grouping exists only so model's field count does not grow by four.
//
// Only the last turn is retained. Holding every turn's reasoning for a session
// that may run for hours is a transcript this client deliberately does not
// keep — see say — and the press that expands is one somebody makes about the
// answer they are looking at, not about one from twenty minutes ago.
//
// begun is the only per-turn field, and flush clears it. That keeps every hook
// this needs inside this file: send and settle live in run.go and would
// otherwise each grow a line to reset a clock they have no other reason to
// know about.
type thoughts struct {
	begun    time.Time
	ended    time.Time
	retained string
	expanded bool
	hinted   bool
}

// stamp records when this turn's reasoning began, the first time there is any
// of it to see.
//
// The honest start is the first KindThinking delta, and absorb is where those
// land — in view.go, which nothing here writes to. streaming() is the next
// thing to run after every absorbed delta, because Bubble Tea draws after
// every message, so the first draw that finds the buffer non-empty is the
// first delta plus one frame. That is a stamp a few hundred microseconds late
// on a figure printed to a tenth of a second.
//
// The zero check is what makes it a start rather than a clock: called on every
// frame of a run, it must only ever fire on the first.
func (m *model) stamp() {
	if m.begun.IsZero() && m.run.reasoning.Len() > 0 {
		m.begun = time.Now()
	}
}

// thought stops the clock, and is called the first time a turn produces
// anything that is not reasoning.
//
// The reasoning is not committed when it ends — flush runs at the end of the
// turn, which is after the answer has streamed and, when the model called a
// tool, after that tool has run. Measuring to flush therefore billed both of
// those to thinking: 0.6s of reasoning followed by a 0.6s answer was reported
// as "thought for 1.2s", measured. The last thing before the first answer
// delta or the first tool call is the honest end, and both of those reach
// absorb.
//
// Only the first one counts. A turn is thinking, then answering, and the
// deltas of that answer arrive a few characters at a time — restamping on each
// would make the end the start of the last word instead of the first.
func (m *model) thought() {
	if !m.begun.IsZero() && m.ended.IsZero() {
		m.ended = time.Now()
	}
}

// elapsed is how long this turn spent thinking, and zero when nothing stamped
// a start.
//
// Zero is not "instant", it is "not measured" — a model driven entirely
// through Update with no frame between the deltas and the flush, which is
// every test that does not draw. collapsed prints no duration for it rather
// than an invented 0.0s, because a number nobody measured is worse than no
// number.
//
// A turn that ended without ever leaving reasoning — abandoned mid-thought, or
// cut off at the token limit — has no end stamped, and is measured to now.
// That is the one case where the answer has not begun, so nothing else has
// been billed to it.
func (m *model) elapsed() time.Duration {
	if m.begun.IsZero() {
		return 0
	}
	if m.ended.IsZero() {
		return time.Since(m.begun)
	}
	return m.ended.Sub(m.begun)
}

// forget drops the reasoning being held for ctrl+t, and is what /clear reaches.
//
// A cleared session is one somebody wanted gone from the screen, and a key
// that reprints the last thing the model thought before they cleared is that
// session coming back. The mode itself survives: expanded is a preference
// about how this client draws, not part of the session it drew.
func (m *model) forget() {
	m.begun, m.ended = time.Time{}, time.Time{}
	m.retained = ""
}

// collapsed is the one line a turn's reasoning becomes: quiet, and carrying
// only the fact that thinking happened and roughly how long it took.
//
// The hint is shown once per session and then never again. A key worth
// discovering is worth one line saying so; the same eight words repeated under
// every answer turns the quietest row on the screen into the one that repeats
// itself, which is the opposite of what collapsing was for. reveal marks it
// shown too — somebody who has already pressed the key does not need telling
// it exists.
func (m *model) collapsed(spent time.Duration) string {
	line := "▶ thought"
	if spent > 0 {
		line += " for " + roughly(spent)
	}
	if !m.hinted {
		m.hinted = true
		line += " · ctrl+t to expand"
	}
	return line
}

// roughly is a thinking duration as a reader reads it: tenths under verbose,
// whole seconds over. See verbose for why the line changes shape there.
func roughly(spent time.Duration) string {
	if spent >= verbose {
		return fmt.Sprintf("%ds", int(spent.Round(time.Second)/time.Second))
	}
	return fmt.Sprintf("%.1fs", spent.Round(100*time.Millisecond).Seconds())
}

// reveal is ctrl+t: it prints the reasoning that was collapsed, and switches
// what every later turn does with its own.
//
// Sticky on purpose. Somebody who presses this once wants to read the model's
// thinking for this session, not to be asked the same question after every
// answer — so the press sets a mode, and pressing it again puts the mode back.
//
// Expanding prints the retained text now, underneath, rather than replacing
// the collapsed line above it. Nothing printed can be replaced: a finished
// line belongs to the terminal's scrollback from the moment it is written, and
// this client has no viewport to redraw — see say. A second copy below is the
// only honest shape for "expand" here, and it is the shape a pager has anyway.
//
// The press is always claimed and always says something. A binding that
// silently does nothing when there is nothing retained is indistinguishable
// from a terminal that dropped the key, and this one has a real answer for
// that case: the mode changed, and the next turn will show it. That is the
// difference from escaped, which reports its idle press unhandled because esc
// belongs to the prompt when this client has nothing to do with it — ctrl+t
// does not, so nothing downstream is being robbed.
func (m *model) reveal() (bool, tea.Cmd) {
	m.expanded = !m.expanded
	m.hinted = true

	switch {
	case m.expanded && m.retained != "":
		m.say(fromThinking, m.retained)
	case m.expanded:
		m.say(fromClient, "reasoning will be shown in full from here")
	default:
		m.say(fromClient, "reasoning will collapse to a single line from here")
	}
	return true, nil
}

// flush moves whatever is still streaming into the transcript, and reports the
// answer it committed so the caller can put that same text in the conversation.
//
// It is not bookkeeping. Both buffers are drawn by render only while they are
// filling, so clearing one without moving its text somewhere permanent erases
// it from the screen at the exact moment it finished — which is what this did
// until it was used.
//
// Reasoning is retained here and collapsed to a line, not printed whole. A
// chain of thought is longer than the answer it produced often enough that
// printing both makes the transcript a place the answer is hidden in, and it
// is the answer the window is for. The text is kept so ctrl+t can print it,
// and kept only in memory: see thoughts.
//
// It is still not recorded in the conversation. nacelle.Reasoning exists and
// would hold it, but what this buffer holds is the readable copy and not the
// one a provider will take back. Within a run the backend already carries the
// real thing on the assistant message it rebuilds, in the shape the provider
// signs and validates; across runs no provider wants a chain of thought from
// an answer it has already given, and both backends drop the part on the way
// out. Recording it here would fill the conversation with something that is
// only ever skipped.
func (m *model) flush() string {
	reasoning := m.run.reasoning.String()
	m.run.reasoning.Reset()

	// reasoningFull holds every line already committed to scrollback during
	// streaming; the remaining partial line from the buffer goes after it.
	if reasoning != "" || m.run.reasoningFull.Len() > 0 {
		spent := m.elapsed()
		m.begun, m.ended = time.Time{}, time.Time{}
		m.retained = m.run.reasoningFull.String() + reasoning
		m.run.reasoningFull.Reset()

		if m.expanded {
			// Only the remaining partial line needs printing now — full committed
			// lines were already printed during streaming via commitReasoning.
			if reasoning != "" {
				m.say(fromThinking, reasoning)
			}
		} else {
			m.say(fromThinking, m.collapsed(spent))
		}
	}

	answer := m.run.fullAnswer.String()
	m.run.answer.Reset()
	m.run.fullAnswer.Reset()
	if answer != "" {
		m.say(fromModel, answer)
	}
	return answer
}
