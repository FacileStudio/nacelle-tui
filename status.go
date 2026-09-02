package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/FacileStudio/nacelle"
)

// abandoned is the run the user stopped.
//
// The core reports why a run ended on KindDone, and cancelling is the one
// ending that arrives without one — the event never comes. It is also the only
// abandonment the reader caused themselves, so it is the last one they should
// have to guess at from a status line still saying "ready" under half an
// answer.
const abandoned nacelle.Stop = "abandoned"

// rephrase is how long one way of saying "nothing has come back yet" stays up
// before the next way replaces it.
//
// It is several spinner cycles long on purpose. The spinner already proves the
// program is alive frame by frame, so words changing at the spinner's own rate
// would read as flicker rather than as time passing. Four seconds is long
// enough to read the line twice and short enough that a wait worth worrying
// about never shows the same words for the whole of it.
const rephrase = 4 * time.Second

// waiting is every way this client says it is waiting on the model.
//
// One fixed sentence was indistinguishable from a frozen line as soon as the
// spinner scrolled past the edge of attention. The question a reader is
// actually asking a minute in is not "is the animation running" but "has
// anything changed since I last looked", and a character cycling in place
// answers the first and not the second — the words are what answer the second.
//
// Every phrase here has to be true at any instant of a run, not only once a
// run has gone on a while, because which one shows is a function of the clock
// rather than of how long this particular wait has lasted. That rules out the
// obvious "still waiting", which would be a lie two hundred milliseconds in.
var waiting = []string{
	"waiting for a response",
	"waiting on the model",
	"waiting on the backend",
}

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

func (m *model) working() string {
	if m.compacting {
		return m.theme.compacting.Render(m.spin.View() + " compacting context")
	}
	doing := waitingVerb(time.Since(m.run.began))
	tone := m.theme.waiting
	switch n := m.running(); n {
	case 0:
	case 1:
		name, ok := m.runningName()
		if ok {
			doing = "running " + name
			tone = toolTone(name)
		}
	default:
		doing = fmt.Sprintf("running %d tools", n)
		tone = m.theme.tool
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
func (m *model) running() int {
	n := 0
	for _, g := range m.run.groups {
		if g.end.IsZero() {
			n++
		}
	}
	return n
}

// runningName is the tool behind the single running call, for the status line.
// The second return is false when there is no single call, in which case the
// caller says "running N tools" instead.
func (m *model) runningName() (string, bool) {
	for _, g := range m.run.groups {
		if g.end.IsZero() {
			return g.tool.Name, true
		}
	}
	return "", false
}

// waitingVerb is the phrase for one moment of this run.
//
// It buckets the time since the run began instead of counting frames, which
// makes it a function of an instant and nothing else. Counting would need
// somewhere to keep the count, and a count kept anywhere outside the run would
// carry into the next wait and start it mid-rotation; kept inside the run it
// would be a second thing to reset in a function that already resets six.
// Bucketing from the run's own start also means every wait opens on the first
// phrase — one bucketed from the wall clock would open wherever the epoch
// happened to be, saying "waiting on the backend" two hundred milliseconds in.
// Nothing here asks for a tick of its own either — the spinner's own tick is
// what redraws the line, so the words change on the first frame after a bucket
// rolls over.
func waitingVerb(elapsed time.Duration) string {
	return waiting[int(elapsed/rephrase)%len(waiting)]
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
func (m *model) ongoing() string {
	if !m.run.busy || m.run.began.IsZero() {
		return ""
	}
	return lasted(time.Since(m.run.began))
}

// shortTokens renders a token count the way a status line wants it: exact
// below a thousand, one decimal up to a hundred thousand, whole thousands
// above that, and millions at the same precision. The precision is decoration;
// nobody audits the third digit of a cache read.
func shortTokens(n int64) string {
	switch {
	case n >= 100_000_000:
		return fmt.Sprintf("%dM", n/1_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 100_000:
		return fmt.Sprintf("%dk", n/1000)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return strconv.FormatInt(n, 10)
	}
}
