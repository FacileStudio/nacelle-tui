package main

import (
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// This file is how a run ends: the state left behind is tidied, what streamed
// is committed, and whatever was typed while it ran is delivered. Starting one
// and stopping one live in run.go — the split is a filet file-length cap
// rather than a new idea, and this was the seam already there.

// settle ends a run: whatever streamed is committed, what it cost joins the
// session total, and the prompt opens again. Usage is folded into spent
// here, not left standing, so the next question starts from a clean counter
// and the status line keeps showing a total that only grows.
//
// pending and running are cleared here too, not only on their own paths: a
// stream that closes without yielding anything never reaches either one, and
// nothing should be left asking a question, or named as still running, about
// a dead run. Clearing running says the lines it was holding for calls no
// result ever answered, so a run that ends mid-tool still reports the tool.
//
// stranded runs after closeTurn because closeTurn flushes the text the model
// streamed before asking for the tool: saying the held line first put the tool
// above the sentence announcing it. consume gets that order right by recording
// before it absorbs, and this is the path that never reaches consume.
//
// Whatever was typed while this run was going is delivered last, once the
// state above is clean — send is what the dispatch reaches, and it would
// otherwise be handed a run that has not finished tidying up after itself.
// One at a time, because dispatching the next queued line starts a run that
// settles again and takes the one after it.
func (m *model) settle() tea.Cmd {
	m.run.cancel()
	m.run.busy = false
	m.run.pending = nil

	m.closeResults()
	m.dropUnanswered()
	m.closeTurn(m.run.stop)
	m.stranded()
	m.sayNothingCame()
	m.spent = m.spent.Add(m.run.usage)
	m.run.usage = nacelle.Usage{}
	m.compact()

	return m.deliver()
}

// deliver sends what was typed while the last run was going, and keeps going
// until something is actually running again.
//
// The loop is the whole point. dispatch has two outcomes and only one of them
// leads back here: a question starts a run, which settles and takes the next
// line, but a command answers on the spot and starts nothing. Popping a single
// line per settle therefore stranded everything behind the first queued
// /help — still drawn as queued, still holding its row, never sent and with
// nothing left that would ever send it, which is precisely the disappearing
// input this queue exists to prevent.
//
// Every command's own Cmd is collected rather than dropped, so a queued /quit
// still quits, and the loop stops on run.busy rather than on a non-nil Cmd —
// a command that returns work to do is not a command that started a run, and
// reading it as one would strand the queue all over again.
//
// Sequence, not Batch, for the reason prints() and route() both give: a batch
// makes no promise about the order its commands run in, and /clear returns one
// that prints. Two queued /clears under a batch can interleave their blank
// runs with each other's transcript, which is a scrambled screen rather than a
// cleared one. Batch was safe only while every command returned something
// order-independent, and that stopped being true without the type changing.
//
// What makes sequencing safe here is the break above: send's own Cmd blocks on
// the results channel, so a run-starting line must be the last thing in cmds
// or everything behind it would wait for the run's first event. Breaking on
// run.busy is what guarantees that, and it is now load-bearing twice.
func (m *model) deliver() tea.Cmd {
	var cmds []tea.Cmd
	for {
		at := m.nextToSend()
		if at < 0 {
			break
		}
		next := m.run.queued[at]
		m.run.queued = append(m.run.queued[:at], m.run.queued[at+1:]...)
		m.layout(m.windowHeight)

		cmds = append(cmds, m.dispatch(next))
		if m.run.busy {
			break
		}
	}
	return tea.Sequence(cmds...)
}

// nextToSend is the first queued line that is free to go out, or -1 when none
// is.
//
// It skips the line being edited rather than sending it. A run settling while
// somebody is halfway through rewriting a queued question would otherwise send
// the version they are in the middle of replacing — the one thing they have
// already decided is wrong — and their edit would then arrive as a second
// message saying almost the same thing. It stays queued until enter says what
// it now is.
//
// Front-first otherwise, because the queue is the order the questions were
// typed in and that is the order they were meant to be asked in.
func (m *model) nextToSend() int {
	editing := m.editing()
	for i := range m.run.queued {
		if i != editing {
			return i
		}
	}
	return -1
}

// sayNothingCame reports a run that ended without ever putting anything on
// screen, which is otherwise the one ending this client cannot tell the reader
// about.
//
// A stream that yields a turn and a done and no text between them is
// well-formed. Nothing errored, nothing was refused, the stop reason says the
// model finished — so cutShort has nothing to say and the status line goes back
// to "ready" under a transcript holding only the question. Measured against
// openrouter/stealth-ox-alpha on 2026-08-24: two events, zero tokens, no error,
// and a client that looked like it had simply ignored the question.
//
// Zero tokens either way is the tell worth passing on. A model that answers
// with nothing still bills for reading the prompt, so a turn that bills for
// neither is a request the provider dropped before running it — which is what
// a model that will not accept this client's tool definitions does, and the
// reader cannot guess that from an empty screen.
//
// Both lines are short enough not to wrap, and neither says whose fault it is.
// The first draft explained that a model refusing tool definitions is the usual
// cause, which was true, wrapped onto a second row, and read as the client
// making an excuse for itself. What a reader can act on is the next thing to
// try, so that is what it says; the diagnosis stays here, where it is wanted by
// whoever is fixing this rather than by whoever is waiting on an answer.
//
// It runs after closeTurn so that a run which committed something is already
// committed, and it says nothing when the run was abandoned or cut short: those
// endings have their own words in cutShort, and two explanations of one silence
// is worse than none.
func (m *model) sayNothingCame() {
	if m.run.reported || cutShort(m.run.stop) != "" {
		return
	}
	if m.run.usage.InputTokens == 0 && m.run.usage.OutputTokens == 0 {
		m.say(fromFailure, "no answer · nothing billed — try another model")
		return
	}
	m.say(fromFailure, "no answer · the turn ended empty")
}
