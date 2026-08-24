package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// cost is what typing /cost says: the session's running total, on demand,
// without ending anything to see it.
//
// The recap prints the same figures on the way out and the status line shows
// part of them while a run is going, but both answer somebody else's moment —
// one arrives only at quit, the other is cut to whatever width the terminal
// has left. This is the same accounting read at the moment the reader asks for
// it, printed into the transcript like anything else said.
//
// The total is spent plus the run in flight, the sum footer and recap already
// agreed on: quitting mid-answer is billed mid-answer, so a total that dropped
// the run in flight would report less than the line beside it.
//
// Zeroes are left out rather than counted. Cost follows the call status.go
// made — shown only when a backend reported one — and failed tools are news
// only when there have been any. A session that has done nothing at all says
// so plainly instead of printing an empty table.
func (m *model) cost() tea.Cmd {
	total := m.spent.Add(m.run.usage)
	pieces := []string{
		"session · " + lasted(time.Since(m.began)),
	}
	if m.tools > 0 {
		pieces = append(pieces, countedNoun(m.tools, "tool"))
	}
	if m.failed > 0 {
		pieces = append(pieces, fmt.Sprintf("%d failed", m.failed))
	}
	pieces = append(pieces,
		"in "+shortTokens(total.InputTokens+total.CacheCreationTokens),
		"out "+shortTokens(total.OutputTokens))
	if total.CacheReadTokens > 0 {
		pieces = append(pieces, shortTokens(total.CacheReadTokens)+" cached")
	}
	if total.Cost > 0 {
		pieces = append(pieces, fmt.Sprintf("$%.4f", total.Cost))
	}

	m.say(fromClient, strings.Join(pieces, " · "))
	return nil
}
