package tui

import (
	"fmt"
	"strings"

	"github.com/FacileStudio/nacelle-tui/internal/status"
)

// parallelTasksView returns a view of the parallel subagent tasks.
func (m *Model) parallelTasksView() string {
	if len(m.parallelTasks) == 0 {
		return ""
	}
	spend := m.parallelSpend()
	var lines []string
	for _, tasks := range m.parallelTasks {
		lines = append(lines, callLines(tasks, max(m.width, 1), spend)...)
	}
	return strings.Join(lines, "\n")
}

// parallelSpend is the run's combined token burn, in the same ↑in ↓out shape
// the status footer uses, with a cost when the backend reported one. Every
// nested agent reports its spend through DelegateUsage, so this covers the
// whole fan-out rather than just the parent's own turns.
func (m *Model) parallelSpend() string {
	total := m.spent.Add(m.run.usage)
	parts := []string{
		"↑" + status.ShortTokens(total.InputTokens+total.CacheCreationTokens),
		"↓" + status.ShortTokens(total.OutputTokens),
	}
	if total.Cost > 0 {
		parts = append(parts, fmt.Sprintf("$%.4f", total.Cost))
	}
	return strings.Join(parts, " ")
}

// parallelTasksRows returns the number of parallel task rows for layout.
func (m *Model) parallelTasksRows() int {
	return parallelTaskRows(m.parallelTasks)
}
