package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

func TestSubMillisecondCallFlooredToMillisecond(t *testing.T) {
	if got := took(400_000); got != "1ms" {
		t.Errorf("took(400µs) = %q, want 1ms", got)
	}
}

func TestDiscardedCallExcludedFromTally(t *testing.T) {
	m := bareBanner()

	m.finished(&nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"x"}`})
	m.finished(&nacelle.ToolEvent{ID: "b", Name: "run_command", Input: `{"command":"x"}`, Err: errors.New("nope")})
	m.finished(&nacelle.ToolEvent{ID: "c", Name: "read_file", Input: `{"path":"y"}`, Discarded: true})

	if m.tools != 2 {
		t.Errorf("counted %d tools, want 2", m.tools)
	}
	if m.failed != 1 {
		t.Errorf("counted %d failures, want 1", m.failed)
	}
}

func TestTheStatusLineNamesTheToolThatIsRunning(t *testing.T) {
	m := sized()
	m.run.busy = true

	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: "a", Name: "read_file"}})

	if status := visible(m.status()); !strings.Contains(status, "running read_file") {
		t.Errorf("status = %q, want it naming the tool between its call and its result", status)
	}
}

func TestTheStatusLineCountsSeveralToolsAtOnce(t *testing.T) {
	m := sized()
	m.run.busy = true

	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: "a", Name: "read_file"}})
	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: "b", Name: "search_content"}})

	if status := visible(m.status()); !strings.Contains(status, "running 2 tools") {
		t.Errorf("status = %q, want it counting both calls in flight", status)
	}
}

func TestAToolStopsBeingNamedOnceItsResultArrives(t *testing.T) {
	m := sized()
	m.run.busy = true

	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: "a", Name: "read_file"}})
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{ID: "a", Name: "read_file"}})

	if status := visible(m.status()); strings.Contains(status, "running read_file") {
		t.Errorf("status = %q, want the finished tool no longer named as running", status)
	}
}

func TestSettleForgetsToolsThatNeverAnswered(t *testing.T) {
	m := sized()
	m.run.cancel = func() {}
	m.run.busy = true
	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: "a", Name: "read_file"}})

	m.settle()

	if n := m.running(); n != 0 {
		t.Errorf("running = %v, want nothing left named as running after the run ended", n)
	}
}
