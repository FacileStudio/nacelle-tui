package tui

import (
	"errors"
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
