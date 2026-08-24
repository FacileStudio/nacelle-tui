package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

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
func (m *model) status() string {
	state := "ready"
	if cut := cutShort(m.run.stop); cut != "" {
		state = cut
	}
	if m.run.busy {
		state = m.working()
		if time.Since(m.run.interrupted) < forceQuit {
			state = "stopping · ctrl+c or ctrl+\\ to quit now"
		}
	}
	if m.run.pending != nil {
		state = fmt.Sprintf("approve %s(%s)? y = once · a = always this session · n = deny",
			m.run.pending.name, truncate(string(m.run.pending.input), 60))
	}

	line := strings.Join(append([]string{state}, m.footer()...), " · ")
	return lipgloss.NewStyle().Faint(true).Render(truncate(line, max(m.width, 1)))
}

// footer is what the session has spent so far, as the pieces the status line
// puts after the state.
//
// The total is spent plus the run in flight, which is the session total.
// Anything else jumps around: a per-run counter that survives into the next
// run reads as the old total plus the new turns and then falls back to the new
// total on its own, which is a number nobody can act on.
//
// The count separates input from output, because in an agentic session they
// are different animals: every turn re-bills the whole conversation as input,
// so a short chat with many tool calls shows an input figure that dwarfs the
// words actually on screen. One merged number reads as a counting bug; two
// numbers read as what they are.
//
// How long the run in flight has been going leads everything, and only while
// there is one. The line is cut to the terminal's width from the right, so
// this order is a priority order, and a reader watching a slow tool is asking
// whether it is wedged — a question no other figure on the line answers, and
// the one that stops mattering the instant the run ends. Idle it is absent
// rather than frozen at its last value, because a stopped clock left on screen
// is a number that looks live and is not.
//
// Cost comes next, ahead of the counts it summarises. Same priority argument:
// the figure a reader is answerable for is worth more of a narrow terminal
// than the tokens behind it. It is still only shown when a backend reports one:
// Anthropic returns tokens and nothing else, and a zero beside a currency
// symbol reads as free rather than as unknown.
//
// Pieces rather than one buffer, so the separator is decided once. Appended
// onto a string each optional piece had to carry its own " · ", which is one
// place per piece to lose it or to double it — invisible in a faint grey line
// until someone reads the line closely enough to count the dots.
func (m *model) footer() []string {
	total := m.spent.Add(m.run.usage)

	var spent []string
	if since := m.ongoing(); since != "" {
		spent = append(spent, since)
	}
	if total.Cost > 0 {
		spent = append(spent, fmt.Sprintf("$%.4f", total.Cost))
	}
	spent = append(spent,
		"in "+shortTokens(total.InputTokens+total.CacheCreationTokens),
		"out "+shortTokens(total.OutputTokens))
	if total.CacheReadTokens > 0 {
		spent = append(spent, shortTokens(total.CacheReadTokens)+" cached")
	}
	if m.trimmed > 0 {
		spent = append(spent, fmt.Sprintf("%d trimmed", m.trimmed))
	}
	return spent
}

// working is what a busy run is doing, spun so the line moves even when
// nothing else on screen does.
//
// The spinner lives on the status line because that is a row this client
// still owns. It covered only the gap before the first event once: after that
// the screen went still for every gap that followed — a tool running, the
// model called again with its result — and a client that has stopped moving
// is indistinguishable from one that has stopped working.
//
// Naming the tool is the difference between knowing something is happening
// and knowing what. A run_command waiting on a slow build and a wedged client
// look identical without it, and this client's own timeout is measured in
// minutes.
//
// What run.running holds is the whole line the call will print, so the name is
// cut back off it here. That is one map rather than two for a reason bigger
// than tidiness: the held line and the tool named as running are the same fact
// about the same call, and two maps are two places to forget it.
//
// With no tool to name there is nothing to say but that the model has not
// answered, so that is the half that rewords itself — see waiting for what a
// line that never changed its words was mistaken for.
func (m *model) working() string {
	doing := waitingVerb(time.Since(m.run.began))
	switch len(m.run.running) {
	case 0:
	case 1:
		for _, line := range m.run.running {
			name, _, _ := strings.Cut(line, "(")
			doing = "running " + name
		}
	default:
		doing = fmt.Sprintf("running %d tools", len(m.run.running))
	}
	return m.spin.View() + " " + doing
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
// above that. The precision is decoration; nobody audits the third digit of
// a cache read.
func shortTokens(n int64) string {
	switch {
	case n >= 100000:
		return fmt.Sprintf("%dk", n/1000)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return strconv.FormatInt(n, 10)
	}
}
