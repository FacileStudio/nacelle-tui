package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// relaxAfterDispatch cancels a busy parent run the moment a model-callable
// parallel fan-out registers, so the turn returns to ready and the parallel_agents
// grind under a live prompt instead of the model pecking at work it cannot read.
func TestRelaxAfterDispatchCancelsABusyRun(t *testing.T) {
	m := sized()
	cancelled := false
	m.run.busy = true
	m.run.cancel = func() { cancelled = true }

	m.relaxAfterDispatch()

	if !cancelled {
		t.Error("a busy run that just dispatched a fan-out was not cancelled")
	}
}

// Then the parent has actually returned to ready — a dispatch from an idle
// prompt (a /parallel fan-out, or a turn already settled) is left alone, because
// there is no run to end.
func TestRelaxAfterDispatchLeavesAnIdlePromptAlone(t *testing.T) {
	m := sized()
	cancelled := false
	m.run.busy = false
	m.run.cancel = func() { cancelled = true }

	m.relaxAfterDispatch()

	if cancelled {
		t.Error("an idle prompt was cancelled — only a busy dispatch-turn should stop")
	}
}

// When parallel agent rows sit under the prompt, one blank line must separate
// the prompt from the agent block so the two do not read as one column of text.
func TestTUIModeSpacesTheAgentsFromThePrompt(t *testing.T) {
	m := tuiModel()
	m.parallelTasks = map[string][]parallelTaskInfo{
		"detach1": {{Task: "research the docs", Active: true, Began: time.Now()}},
	}
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	promptRow, agentRow := -1, -1
	for i, ln := range lines {
		if strings.Contains(ln, "placeholder") {
			promptRow = i
		}
		if strings.Contains(ln, "research the docs") {
			agentRow = i
		}
	}
	if promptRow < 0 || agentRow < 0 {
		t.Fatalf("no prompt or agent row in\n%s", strings.Join(lines, "\n"))
	}
	if agentRow != promptRow+2 {
		t.Errorf("agent at row %d, prompt at %d, want one blank row between them", agentRow, promptRow)
	}
	if strings.TrimSpace(lines[agentRow-1]) != "" {
		t.Errorf("row between prompt and agents = %q, want blank", lines[agentRow-1])
	}
}
