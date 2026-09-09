package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// handleParallelCommand parses a `/parallel task1, task2, task3` line and
// hands the model a prompt that asks it to use the parallel_subagent tool.
// The actual fan-out lives in nacelle.NewParallelSubAgentTool; this command
// is the surface that lets the user invoke it without asking the model to
// think of doing so itself.
//
// Tasks are split on `,`, trimmed, and empty entries dropped. A single task
// still routes through the parallel tool — its input is a task list, so a
// one-item list is the smallest call.
func (m *Model) handleParallelCommand(args string) tea.Cmd {
	tasks := splitParallelTasks(args)
	if len(tasks) == 0 {
		m.say(fromClient, "usage: /parallel task1, task2, task3")
		return nil
	}
	prompt := buildParallelPrompt(tasks)
	return m.send(prompt)
}

// splitParallelTasks turns `/parallel a, b , c` into ["a", "b", "c"].
func splitParallelTasks(args string) []string {
	parts := strings.Split(args, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// buildParallelPrompt is the message the parent agent sees when the user
// types `/parallel`. It names the tool explicitly and lists the tasks the
// way the tool's schema wants them.
func buildParallelPrompt(tasks []string) string {
	var b strings.Builder
	b.WriteString("Run these independent tasks in parallel using the ")
	b.WriteString(nacelle.ParallelSubAgentToolName)
	b.WriteString(" tool, and report each result as it returns:\n")
	for i, t := range tasks {
		fmt.Fprintf(&b, "\n%d. %s", i+1, t)
	}
	return b.String()
}
