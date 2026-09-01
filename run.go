package main

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// inflight is the one run this client allows at a time: how to hear from it,
// how to abandon it, what it has produced, and what it has cost.
//
// Reasoning gets a buffer of its own: sharing the answer's put the last
// thought against the first word with no separator, and worse, committed the
// reasoning to the conversation as prose, re-sending a chain of thought every
// later turn in the one field no provider wants it replayed in. The copy that
// does travel back is assembled inside the backend, where the provider's own
// shape for it survives.
//
// usage is this run alone, because KindTurn accumulates and KindDone
// replaces: a counter carried over from the last run would double-count its
// total against this run's turns.
//
// running is every tool between its call and its result, keyed by call id so
// several at once are counted right — the SDK's own runner executes a turn's
// tools concurrently. It is what lets the status line name what it is waiting
// on instead of only that it is waiting, and it is emptied by settle rather
// than trusted to drain: a run capped at its iteration limit, or abandoned
// mid-tool, reaches no result for the calls it stopped short of.
//
// What it holds against each id is the transcript line that call will print,
// not merely the tool's name — the line waits for the result so the duration
// can fold into it, see toolline.go. Emptying this map therefore has
// consequences on screen, so both places that empty it go through stranded.
//
// queued is what was typed while a run was still going, delivered when it
// settles. Refusing input until a run finishes is the behaviour this replaces,
// and it was worse than it sounds: the prompt silently ignored enter, so the
// only way to know a question had not been asked was to notice nothing
// happening. pi calls this a follow-up message and delivers it the same way.
//
// pending is set only when -approve-tools is on and a call is waiting on a
// decision — nil otherwise, so status() and key() only change behaviour for
// someone who asked for a gate at all.
//
// editState is embedded rather than named so its fields still read as
// m.run.root, m.run.diffs and m.run.edits — the grouping exists only to keep
// inflight's own field count under filet's cap, the same reason clock and
// turn below are embedded.
// A run keeping the full answer as one string alongside the streaming buffer
// means a paragraph committed to scrollback is never lost from the conversation
// the closeTurn appends to — the two buffers serve different consumers.
type inflight struct {
	results    <-chan result
	cancel     context.CancelFunc
	answer     strings.Builder // streaming: partial paragraph shown in the live region
	fullAnswer strings.Builder // conversation: every word the model said this turn
	reasoning  strings.Builder
	usage      nacelle.Usage
	stop       nacelle.Stop
	busy       bool
	pending    *approvalRequest
	queued     []string

	editState

	// clock is embedded rather than named so its fields still read as
	// m.run.began and m.run.interrupted — the grouping exists only to keep
	// inflight's own field count under filet's cap, the same reason turn
	// below and Config's Discovery in config.go are embedded.
	clock

	// turn is embedded rather than named so both fields still read as
	// m.run.asked and m.run.answered — the grouping exists only to keep
	// inflight's own field count from growing every time this list does,
	// the same reason Config embeds Discovery in config.go.
	turn

	// groups is the tool calls of this run folded into rows. It is kept on
	// the run rather than rebuilt from run.tools on every paint because the
	// grouping has to happen as the events arrive: a call that has not
	// returned yet is half a row, and two calls that finish out of order
	// would not merge if grouped after the fact.
	//
	// groupIndex maps a tool ID to its group, so a result can find its row
	// without scanning. The map is small — one entry per in-flight call —
	// and it is cleared with the groups.
	groups     []toolGroup
	groupIndex map[string]int
}

// beginTool turns a call into a group row. A new call either extends the
// previous group — same tool, same input, and the previous one has not
// returned yet — or starts a fresh row. The second case is what makes the
// grouping safe: a result never starts a group, so two results in a row are
// two different calls finishing and they stay separate.
//
// The index is written here rather than in absorb so the caller can decide
// whether grouping is on at all, and so a result can look its row up by ID
// without scanning the slice.
func (r *inflight) beginTool(ev nacelle.ToolEvent, groupTools bool) {
	if r.groups == nil {
		r.groups = make([]toolGroup, 0, 4)
	}
	if r.groupIndex == nil {
		r.groupIndex = make(map[string]int, 4)
	}
	if groupTools && len(r.groups) > 0 {
		g := &r.groups[len(r.groups)-1]
		if g.end.IsZero() &&
			ev.Name == g.tool.Name &&
			ev.Input == g.tool.Input {
			g.count++
			if ev.ID != "" {
				r.groupIndex[ev.ID] = len(r.groups) - 1
			}
			return
		}
	}
	g := toolGroup{name: ev.Name, input: ev.Input, count: 1, tool: ev, start: time.Now()}
	r.groups = append(r.groups, g)
	if ev.ID != "" {
		r.groupIndex[ev.ID] = len(r.groups) - 1
	}
}

// finishTool marks a group's row as returned, with the outcome and the
// duration. It looks the group up by ID rather than by position, because a
// result can arrive for a call that was not the most recent one — the model
// fires several reads in parallel and they land back in whatever order the
// filesystem gives. Position-based matching would attach the duration to the
// wrong row and leave the real one hanging forever.
func (r *inflight) finishTool(ev nacelle.ToolEvent) {
	if ev.ID == "" {
		// No identifier means this is a call the model did not name, which
		// happens when the backend synthesizes one. Fall back to the last
		// unfinished group: there is only ever one of those in flight at a
		// time in that case, so it is the right row.
		for i := len(r.groups) - 1; i >= 0; i-- {
			if r.groups[i].end.IsZero() {
				r.groups[i].tool = ev
				r.groups[i].end = time.Now()
				r.groups[i].failed = ev.Err != nil
				r.groups[i].discarded = ev.Discarded
				return
			}
		}
		return
	}
	i, ok := r.groupIndex[ev.ID]
	if !ok || i >= len(r.groups) {
		return
	}
	r.groups[i].tool = ev
	r.groups[i].end = time.Now()
	r.groups[i].failed = ev.Err != nil
	r.groups[i].discarded = ev.Discarded
}

// heldLine returns the line a finished call will print, and whether the call
// had one. It looks the group up by ID — the same lookup finishTool uses — so
// a result that arrives for a call the model did not name still finds the one
// unfinished row there is. The second return is false when the call was never
// grouped, which happens for a result with no identifier: finished falls back
// to the single-call line in that case.
func (r *inflight) heldLine(id string, width int) (string, bool) {
	if id == "" {
		return "", false
	}
	i, ok := r.groupIndex[id]
	if !ok || i >= len(r.groups) {
		return "", false
	}
	return r.groups[i].groupLine(width), true
}

// clearGroups drops the run's tool rows. It is called from settle and from
// stranded, the same two places that empty running and edits, so the three
// stay in lockstep and a run never inherits another run's tool rows.
func (r *inflight) clearGroups() {
	r.groups = nil
	r.groupIndex = nil
}

// clock is when this run started and when esc was first pressed this one.
type clock struct {
	began       time.Time
	interrupted time.Time
}

// turn is the assistant turn being built for the conversation: the tools it
// asked for, and the results collected to answer them. They are one state
// machine — see conversation.go, which is the only thing that drives them —
// and separate from everything above, which is about the run rather than
// about what will be sent back.
type turn struct {
	asked    []nacelle.Part
	answered []nacelle.Part

	// reported is whether this run put anything on screen: a word of an
	// answer, a tool call, or a failure. It is the one thing settle cannot
	// work out for itself once the run is over — the answer buffer has been
	// flushed, the tool map emptied and the stop reason is the same "ended
	// cleanly" whether the model wrote a page or nothing at all.
	reported bool
}

// editState is what a run tracks per tool call, plus what drawing a diff for
// its file edits needs: the directory the file tools work in, whether diffs
// were asked for at all, and the before/after of every editing call in
// flight, keyed by call id. It lives on the run rather than the model because
// a run's edits are not a property of the client — the same edits map is
// cleared in settle and in stranded, the two places a run ends, so it never
// survives into the next one.
//
// The capture happens when the call is seen rather than when it finishes —
// by then write_file has already replaced the very contents the diff's
// before side needs.
type editState struct {
	root  string
	diffs bool
	edits map[string]editChange
}

// send starts a run over text — a plain question as typed, or a
// /skill:name's own SKILL.md content with whatever followed the name
// appended. Everything from the last run is cleared here, nowhere else: a
// leftover stop reason mislabels this answer truncated, a leftover usage
// double-counts, and a leftover interruption quits a fresh run outright.
//
// running is emptied here rather than only in settle so a run never inherits
// the last one's unanswered calls and reports them as its own — and emptied
// through stranded, so inheriting is not traded for swallowing.
//
// The batch this returns blocks until the model sends its first anything, and
// so does the one consume returns. Neither of them flushes what has been said
// first — Update does, before it runs either. See its doc comment for what
// went wrong while that was the other way round.
func (m *model) send(text string) tea.Cmd {
	m.run.stop = ""
	m.run.usage = nacelle.Usage{}
	m.run.began = time.Now()
	m.run.interrupted = time.Time{}
	m.run.asked, m.run.answered = nil, nil
	m.run.reported = false
	m.stranded()
	m.conversation = append(m.conversation, nacelle.UserText(text))

	ctx, cancel := context.WithCancel(context.Background())
	m.run.cancel = cancel
	m.run.busy = true
	m.run.results = start(ctx, m.agent, m.conversation)
	return tea.Batch(waitFor(m.run.results), m.spin.Tick)
}

// halt stops the run in flight and leaves the queue standing.
//
// It is the half both stop keys share, and all they share: what they disagree
// about is the queue, and what they disagree about with nothing running at all
// — ctrl+c quits the client there, esc does nothing. Stopping itself is one
// behaviour and this is it, in one place, so the two can never drift into
// stopping a run differently.
//
// interrupted is stamped here rather than by either caller. It is what the
// status line reads to say the run is stopping instead of leaving the spinner
// claiming it is still going, and it is what arms the force quit — busy is
// only cleared by settle, settle waits on the results channel, and a tool
// wedged on a subprocess never closes it.
func (m *model) halt() {
	m.run.interrupted = time.Now()
	m.run.stop = abandoned
	m.run.pending = nil
	m.run.cancel()
}

// abandon stops the run in flight and everything it would have led to.
//
// This is ctrl+c's stop, and dropping the queue is the whole of what makes it
// ctrl+c's. Left alone the queue is delivered by settle, which cancelling
// reaches like any other ending, so stopping one run would start the next on
// the spot. That is the wrong answer for a key whose next press quits the
// client, and the right one for esc — see escaped.
func (m *model) abandon() {
	m.halt()
	m.dropQueued()
}

// escaped is esc once the dropdown has had its turn: it stops the run in
// flight and hands the session straight to whatever was queued behind it.
//
// That is the whole difference between the two stop keys. Both end the answer
// being written; esc means "not this one, move on", ctrl+c means "stop". So
// esc leaves the queue alone and lets settle deliver it — the next line starts
// the moment the cancelled stream closes, which is the behaviour anyone who
// queued a follow-up while reading a wrong answer is pressing esc to get.
// Dropping the queue there made the queue useless exactly when it was most
// wanted: the reader had to watch the whole wrong answer out to keep it.
//
// It says so when there is a queue, because otherwise esc reads as having done
// nothing — the spinner keeps spinning, and the only way to tell it is now
// spinning for a different question is to recognise the answer changing under
// it.
//
// Idle it is not this client's key at all, which is why it reports the press
// unhandled rather than swallowing it. Claiming it would turn the one press
// every terminal reader uses to back out of something into a press that
// silently does nothing here, and stop it reaching the prompt.
//
// A run already asked to stop is left alone rather than asked again, and the
// press is still claimed. Asking twice re-stamps run.interrupted, which is
// what holds the force-quit offer open — so an esc held down, or tapped at a
// tool that is not listening, would keep ctrl+c meaning "quit now" for as long
// as the tapping lasted rather than the three seconds it is meant to. Past
// that window a fresh press is a fresh request, and cancels again.
//
// It lives beside abandon rather than in key for the same reason decide and
// navigateMenu do: a binding belongs with the thing it acts on, and this one
// acts on the run.
func (m *model) escaped() (bool, tea.Cmd) {
	if !m.run.busy {
		return false, nil
	}
	if time.Since(m.run.interrupted) >= forceQuit {
		m.halt()
		if waiting := len(m.run.queued); waiting > 0 {
			m.say(fromClient, "stopped · "+countedNoun(waiting, "queued message")+" still to send")
		}
	}
	return true, nil
}

// consume folds one result into the transcript and waits for the next. The
// answer is committed before the error is — an error printed first would
// tell the reader the request failed, then show the half sentence it
// interrupted, the wrong order to read a failure in.
func (m *model) consume(next result) tea.Cmd {
	if next.err != nil {
		m.flush()
		m.run.reported = true
		m.say(fromFailure, next.err.Error())
		return waitFor(m.run.results)
	}
	m.record(next.event)
	m.absorb(next.event)
	return waitFor(m.run.results)
}
