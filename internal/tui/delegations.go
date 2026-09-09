package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

type spentDelegation struct {
	usage nacelle.Usage
}

func watchDelegations() tea.Cmd {
	return func() tea.Msg {
		return spentDelegation{usage: <-delegations}
	}
}

func (m *Model) recordDelegation(spent spentDelegation) tea.Cmd {
	m.run.usage = m.run.usage.Add(spent.usage)
	return watchDelegations()
}
