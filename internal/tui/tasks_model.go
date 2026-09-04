package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle-tui/internal/tasks"
)

// watchTasks waits for one reported plan and hands it to the update loop as a
// message.
func watchTasks() tea.Cmd {
	return func() tea.Msg {
		return <-tasks.Reports
	}
}

// recordTasks stores the plan the model last reported and re-arms the watcher.
func (m *Model) recordTasks(reported tasks.TaskUpdate) tea.Cmd {
	m.tasks = tasks.TaskList(reported)
	m.layout(m.windowHeight)
	return watchTasks()
}
