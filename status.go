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

// status is the one line that is always true: what the session has cost so
// far, whether a run is still going, and whether the answer above it is whole.
//
// It no longer reports being scrolled back, because there is no longer any
// such state to report — the terminal owns scrolling, and a client that has
// been scrolled away from does not know it and does not need to.
//
// The count is spent plus the run in flight, which is the session total.
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
// Cost is only shown when a backend reports one. Anthropic returns tokens and
// nothing else, and a zero next to a currency symbol reads as free rather than
// as unknown.
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

	total := m.spent.Add(m.run.usage)
	line := fmt.Sprintf("%s · in %s · out %s", state,
		shortTokens(total.InputTokens+total.CacheCreationTokens),
		shortTokens(total.OutputTokens))
	if total.CacheReadTokens > 0 {
		line += fmt.Sprintf(" · %s cached", shortTokens(total.CacheReadTokens))
	}
	if total.Cost > 0 {
		line += fmt.Sprintf(" · $%.4f", total.Cost)
	}
	if m.trimmed > 0 {
		line += fmt.Sprintf(" · %d trimmed", m.trimmed)
	}
	return lipgloss.NewStyle().Faint(true).Render(truncate(line, max(m.width, 1)))
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
func (m *model) working() string {
	doing := "waiting for a response"
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
