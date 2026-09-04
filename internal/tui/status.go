package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle-tui/internal/tui/toolview"
)

// abandoned is the run the user stopped.
//
// The core reports why a run ended on KindDone, and cancelling is the one
// ending that arrives without one — the event never comes. It is also the only
// abandonment the reader caused themselves, so it is the last one they should
// have to guess at from a status line still saying "ready" under half an
// answer.
const abandoned nacelle.Stop = "abandoned"

// status is the one line that is always true: what the session has cost so
// far, whether a run is still going, and whether the answer above it is whole.
//
// It no longer reports being scrolled back, because there is no longer any
// such state to report — the terminal owns scrolling, and a client that has
// been scrolled away from does not know it and does not need to.
//
// What the session has spent is footer's, not this function's. Splitting it
// off is what keeps the state — the leftmost thing, and the only part of the
// line a narrow terminal is guaranteed to keep — one decision read in one
// place, rather than the first of six appends to a shared buffer.

func (m *Model) working() string {
	if m.compacting {
		return m.theme.Compacting.Render(m.spin.View() + " compacting context")
	}
	doing := waitingVerb(time.Since(m.run.began))
	tone := m.theme.Waiting
	switch n := m.running(); n {
	case 0:
	case 1:
		name, ok := m.runningName()
		if ok {
			doing = "running " + name
			tone = toolview.ToolTone(name)
		}
	default:
		doing = fmt.Sprintf("running %d tools", n)
		tone = m.theme.Tool
	}
	if since := m.ongoing(); since != "" {
		doing += " · " + since
	}
	return tone.Render(m.spin.View() + " " + doing)
}

// running is the number of tool rows still open in this run — calls that have
// not returned yet. It is the same rows the live region renders, so the status
// line and the transcript can never disagree about how many tools are in
// flight.
func (m *Model) running() int {
	n := 0
	for _, g := range m.run.groups {
		if g.End.IsZero() {
			n++
		}
	}
	return n
}

// runningName is the tool behind the single running call, for the status line.
// The second return is false when there is no single call, in which case the
// caller says "running N tools" instead.
func (m *Model) runningName() (string, bool) {
	for _, g := range m.run.groups {
		if g.End.IsZero() {
			return g.Tool.Name, true
		}
	}
	return "", false
}

// ongoing is how long the run in flight has been going, and the empty string
// when nothing is running.
//
// It measures this run rather than the session. Someone reading it is asking
// whether the tool in front of them is wedged, and a session counter reading
// 41m answers a question nobody asked while hiding the one they did. The
// session's own span is not lost — recap says it on the way out, which is
// where a total belongs.
//
// It is not called running, which is the name the sentence wants, because
// run.running is the map of calls in flight two functions up this same file.
// Two things a line apart called the same thing is how somebody reads the
// wrong one and cannot see why the count is a duration.
//
// It borrows recap's lasted rather than rounding again here. The two are the
// same measurement shown twice, and a client that called the same forty-one
// seconds 41s in one place and 41.0021s in the other would be reporting a
// discrepancy it does not have.
//
// The zero check is not defensive dressing over the busy check. begun is
// stamped by send, which is also the only thing that sets busy, so the two
// agree in this program — but a test that sets busy by hand to draw a status
// line does not go through send, and time.Since on a zero Time renders as a
// span in the thousands of hours. Refusing to print it is cheaper than a rule
// nobody reading a test would know they had broken.
func (m *Model) ongoing() string {
	if !m.run.busy || m.run.began.IsZero() {
		return ""
	}
	return lasted(time.Since(m.run.began))
}

func (m *Model) spun(message spinner.TickMsg) tea.Cmd {
	return m.spin.Spun(message, m.run.busy)
}
