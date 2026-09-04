package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle-tui/internal/tui/cost"
)

func (m *Model) cost() tea.Cmd {
	total := m.spent.Add(m.run.usage)
	m.say(fromClient, cost.Summary(total, m.tools, m.failed, time.Since(m.began)))
	return nil
}
