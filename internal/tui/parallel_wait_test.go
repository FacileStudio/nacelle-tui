package tui

import (
	"testing"
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
