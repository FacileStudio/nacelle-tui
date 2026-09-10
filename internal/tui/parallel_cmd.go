package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// handleParallelCommand parses a `/parallel task1, task2, task3` line and
// launches the fan-out. It does not go through the parent model: the work runs
// in detached nested agents cloned from the main agent's own Config, and the
// main thread stays free — the point of /parallel is that you keep chatting
// while the subagents grind. This is unlike the `parallel_subagent` tool the
// model can still call mid-turn, which blocks the parent until its fan-out
// returns.
//
// Tasks are split on `,`, trimmed, and empty entries dropped. A single task
// still routes through the parallel machinery — its input is a task list, so a
// one-item list is the smallest call.
func (m *Model) handleParallelCommand(args string) tea.Cmd {
	tasks := splitParallelTasks(args)
	if len(tasks) == 0 {
		m.say(fromClient, "usage: /parallel task1, task2, task3")
		return nil
	}
	if m.delegate.Backend == nil {
		m.say(fromFailure, "no agent is configured to run parallel agents")
		return nil
	}
	return m.launchDetached(tasks)
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
