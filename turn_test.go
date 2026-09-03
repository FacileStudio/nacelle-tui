package main

import (
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

func runTwoTurns(m *model) {
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "looking now\n"})
	m.record(nacelle.Event{Kind: nacelle.KindText, Text: "looking now\n"})
	tool := &nacelle.ToolEvent{ID: "call_1", Name: "read_file", Input: `{"path":"main.go"}`}
	m.record(nacelle.Event{Kind: nacelle.KindToolCall, Tool: tool})
	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: tool})
	m.absorb(nacelle.Event{Kind: nacelle.KindTurn, Usage: nacelle.Usage{InputTokens: 10, OutputTokens: 5, Cost: 0.001}})

	res := &nacelle.ToolEvent{ID: "call_1", Name: "read_file", Result: "package main", Duration: 50 * time.Millisecond}
	m.record(nacelle.Event{Kind: nacelle.KindToolResult, Tool: res})
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: res})

	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "all done\n"})
	m.record(nacelle.Event{Kind: nacelle.KindText, Text: "all done\n"})
	m.absorb(nacelle.Event{Kind: nacelle.KindTurn, Usage: nacelle.Usage{InputTokens: 20, OutputTokens: 10, Cost: 0.002}})
	m.absorb(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopEnd})
	m.settle()
}

func TestMultiTurnToolExecutionBoundary(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.cancel = func() {}
	m.conversation = append(m.conversation, nacelle.UserText("read a file"))
	runTwoTurns(m)

	lines := spoken(m)
	if len(lines) < 5 {
		t.Fatalf("spoken lines = %d %v, want at least 5 lines", len(lines), lines)
	}
	if !strings.Contains(lines[0], "looking now") || !strings.Contains(lines[1], "15 tokens") {
		t.Errorf("turn 1 = %v, want text and tokens", lines[:2])
	}
	if !strings.Contains(lines[2], "read_file(main.go)") {
		t.Errorf("tool line = %q, want read_file", lines[2])
	}
	if !strings.Contains(lines[3], "all done") || !strings.Contains(lines[4], "30 tokens") {
		t.Errorf("turn 2 = %v, want text and tokens", lines[3:5])
	}
	if len(m.conversation) != 4 || said(m.conversation[1]) != "looking now\n" || said(m.conversation[3]) != "all done\n" {
		t.Errorf("conversation = %v, want preserved turns", m.conversation)
	}
}

func TestInFlightLineTruncatesWithinPaneWidth(t *testing.T) {
	m := bareBanner()
	m.width = 40
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "1", Name: "run_command", Input: `{"command":"very-long-argument-one-two-three-four-five-six"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "2", Name: "run_command", Input: `{"command":"another-very-long-argument-that-would-overflow"}`}, true)

	g := m.run.groups[0]
	line := g.inFlightLine(m.width)
	if len(line) > m.width+len("…") {
		t.Errorf("inFlightLine len = %d, want <= width (%d)", len(line), m.width)
	}
}
