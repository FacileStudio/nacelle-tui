package main

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// send starts a run over text — a plain question as typed, or a
// /skill:name's own SKILL.md content with whatever followed the name
// appended. Everything from the last run is cleared here, nowhere else: a
// leftover stop reason mislabels this answer truncated, a leftover usage
// double-counts, and a leftover interruption quits a fresh run outright.
func (m *model) send(text string) tea.Cmd {
	m.run.stop = ""
	m.run.usage = nacelle.Usage{}
	m.run.began = time.Now()
	m.run.turnBegan = time.Now()
	m.run.interrupted = time.Time{}
	m.run.asked, m.run.answered = nil, nil
	m.run.reported = false
	m.stranded()
	m.conversation = append(m.conversation, nacelle.UserText(text))

	ctx, cancel := context.WithCancel(context.Background())
	m.run.cancel = cancel
	m.run.busy = true

	if count, err := m.agent.CountTokens(ctx, m.conversation); err == nil && m.compactAt > 0 && count > m.compactAt+compactSlack {
		m.size = count
		m.compact()
	}

	m.run.results = start(ctx, m.agent, m.conversation)
	return tea.Batch(waitFor(m.run.results), m.spin.Tick)
}

// halt stops the run in flight and leaves the queue standing.
func (m *model) halt() {
	m.run.interrupted = time.Now()
	m.run.stop = abandoned
	m.run.pending = nil
	m.run.cancel()
}

// abandon stops the run in flight and drops everything it would have led to.
func (m *model) abandon() {
	m.halt()
	m.dropQueued()
}

// escaped is esc once the dropdown has had its turn: it stops the run in
// flight and hands the session straight to whatever was queued behind it.
// Idle it is not this client's key at all, which is why it reports the press
// unhandled rather than swallowing it.
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

// consume folds one result into the transcript and waits for the next.
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
