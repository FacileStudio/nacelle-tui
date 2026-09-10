package tui

import (
	"context"
	"encoding/json"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// handleParallelCommand parses a `/parallel task1, task2, task3` line and
// launches the fan-out. It does not go through the parent model: the work runs
// in detached nested agents cloned from the main agent's own Config, and the
// main thread stays free — the point of /parallel is that you keep chatting
// while the subagents grind. The model's own `parallel_subagent` tool is the
// same detached deal: it returns a stub immediately and the fan-out streams
// back through the Results hook, so neither path pins the main thread to the
// fan-out's duration.
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

// delegateApprove is the approval policy the detached subagents answer to. It
// is the same rule agent.delegateApprovals applies to the model-callable tool:
// a delegate inherits the parent's policy, and a session with approvals off
// hands the delegate an allow-all rather than the SDK's deny-all default.
// The rule lives here rather than in agent because agent imports tui, so tui
// cannot import it back — this is the mirror that keeps the two linked.
func delegateApprove(cfg nacelle.Config) nacelle.Approve {
	if cfg.Approve != nil {
		return cfg.Approve
	}
	return func(context.Context, string, json.RawMessage) bool { return true }
}
