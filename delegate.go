package main

import (
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// delegations carries what every delegated run spends, from the tool's
// goroutine to the update loop. It is buffered so a delegation is never
// blocked by its own bookkeeping: an unbilled turn is a wrong total, but a
// stalled one is a wedged agent.
var delegations = make(chan nacelle.Usage, 64)

// spentDelegation says the session's totals grew by work the parent never
// streamed — a sub-agent's turns arrive here rather than as KindTurn events,
// because the parent's stream sees only the one tool call.
type spentDelegation struct {
	usage nacelle.Usage
}

// watchDelegations waits for one delegated spend and reports it as a message.
// Every handling re-arms it, so it sits listening for the whole session the
// same way waitFor sits on the run's own results.
func watchDelegations() tea.Cmd {
	return func() tea.Msg {
		usage := <-delegations
		return spentDelegation{usage: usage}
	}
}

// record folds one delegated spend into the run in flight. The footer adds
// that to what the session had already spent, so a delegation's cost shows up
// while it runs instead of only on the receipt.
func (m *model) recordDelegation(spent spentDelegation) tea.Cmd {
	m.run.usage = m.run.usage.Add(spent.usage)
	return watchDelegations()
}
