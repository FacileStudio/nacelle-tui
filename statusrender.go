package main

import (
	"fmt"
	"strings"
	"time"
)

func (m *model) status() string {
	isReady := true
	state := "✓ ready"
	if cut := cutShort(m.run.stop); cut != "" {
		state = cut
		isReady = false
	}
	if m.run.busy {
		state = m.working()
		isReady = false
		if time.Since(m.run.interrupted) < forceQuit {
			state = "stopping · ctrl+c or ctrl+\\ to quit now"
		}
	}
	if m.run.pending != nil {
		state = fmt.Sprintf("approve %s(%s)? y = once · a = always this session · n = deny",
			m.run.pending.name, truncate(unstyled(string(m.run.pending.input)), 60))
	}

	width := max(m.width, 1)
	counts := strings.Join(m.footer(), " · ")
	stateLine := truncate(state, width)
	if isReady {
		stateLine = m.theme.ready.Render(stateLine)
	}
	return stateLine + "\n" + m.theme.muted.Render(truncate(counts, width))
}

// footer is what the session has spent so far, as the pieces the counts row
// puts under the state.
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
	if total.Cost > 0 {
		spent = append(spent, fmt.Sprintf("$%.4f", total.Cost))
	}
	spent = append(spent,
		"in "+shortTokens(total.InputTokens+total.CacheCreationTokens),
		"out "+shortTokens(total.OutputTokens))
	if total.CacheReadTokens > 0 {
		spent = append(spent, shortTokens(total.CacheReadTokens)+" cached")
	}
	if m.size > 0 {
		spent = append(spent, "ctx "+shortTokens(m.size))
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
//
// The spinner, the phrase and the clock are rendered as one span in one
// colour, and the colour is the phase: cyan while the model is being waited
// on, the tool's own colour once something runs, purple while a compaction is
// in progress. Two of the three used to be styled separately and the spinner
// not at all, which read as three widgets sharing a row rather than one
// sentence about one run — and left the busiest line on screen in the same
// grey as everything that had finished. Colouring the whole span means the
// frame where the colour changes is itself the statement that the phase
// changed, before anybody has read the words.
