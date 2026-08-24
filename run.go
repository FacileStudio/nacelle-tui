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
type inflight struct {
	results   <-chan result
	cancel    context.CancelFunc
	answer    strings.Builder
	reasoning strings.Builder
	usage     nacelle.Usage
	stop      nacelle.Stop
	busy      bool
	pending   *approvalRequest
	queued    []string

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
}

// editState is what a run tracks per tool call, plus what drawing a diff for
// its file edits needs: the directory the file tools work in, whether diffs
// were asked for at all, and the before/after of every editing call in
// flight, keyed by call id alongside the transcript line each call will
// print. running lives here with edits because both are emptied by the same
// two places — settle and stranded — and neither survives its run.
//
// The capture happens when the call is seen rather than when it finishes —
// by then write_file has already replaced the very contents the diff's
// before side needs.
type editState struct {
	root    string
	diffs   bool
	running map[string]string
	edits   map[string]editChange
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
	m.stranded()
	m.conversation = append(m.conversation, nacelle.UserText(text))

	ctx, cancel := context.WithCancel(context.Background())
	m.run.cancel = cancel
	m.run.busy = true
	m.run.results = start(ctx, m.agent, m.conversation)
	return tea.Batch(waitFor(m.run.results), m.spin.Tick)
}

// abandon stops the run in flight and everything it would have led to.
//
// It is what both stop keys do, and it is all they have in common: they differ
// only in what they mean when there is nothing to stop, where ctrl+c quits the
// client and esc does nothing at all. Stopping itself is one behaviour and
// this is it, in one place, so the two can never drift into stopping a run
// differently.
//
// interrupted is stamped here rather than by either caller. It is what the
// status line reads to say the run is stopping instead of leaving the spinner
// claiming it is still going, and it is what arms the force quit — busy is
// only cleared by settle, settle waits on the results channel, and a tool
// wedged on a subprocess never closes it.
//
// The queue goes with the run. Left alone it is delivered by settle, which
// cancelling reaches like any other ending, so stopping one run would start
// the next one on the spot — the opposite of what either key was pressed for.
func (m *model) abandon() {
	m.run.interrupted = time.Now()
	m.run.stop = abandoned
	m.run.pending = nil
	m.run.cancel()
	m.dropQueued()
}

// escaped is esc once the dropdown has had its turn: it stops a run in flight,
// and does nothing else ever.
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
		m.abandon()
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
		m.say(fromFailure, next.err.Error())
		return waitFor(m.run.results)
	}
	m.record(next.event)
	m.absorb(next.event)
	return waitFor(m.run.results)
}
