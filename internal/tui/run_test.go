package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

func ranFor(spent time.Duration) *model {
	m := bareBanner()
	m.began = time.Now().Add(-spent)
	return m
}

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

func TestDelegatedSpendJoinsTheRunTotal(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.usage = nacelle.Usage{InputTokens: 100}

	cmd := m.recordDelegation(spentDelegation{usage: nacelle.Usage{InputTokens: 40, OutputTokens: 7}})
	if got := m.run.usage.InputTokens; got != 140 {
		t.Fatalf("input = %d, want 140", got)
	}
	if cmd == nil {
		t.Fatal("handling spend did not re-arm watcher")
	}
}

func TestARunThatSaidNothingSaysSo(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.cancel = func() {}
	m.settle()

	said := strings.Join(spoken(m), "\n")
	if !strings.Contains(said, "no answer") {
		t.Errorf("transcript = %q, want empty run reported", said)
	}
}

func TestARunThatSaidSomethingIsNotReportedAsEmpty(t *testing.T) {
	for _, tc := range []struct {
		name     string
		reported bool
		stop     nacelle.Stop
	}{
		{"said something", true, ""},
		{"abandoned", false, abandoned},
	} {
		m := sized()
		m.run.busy = true
		m.run.cancel = func() {}
		m.run.reported, m.run.stop = tc.reported, tc.stop
		m.settle()

		if said := strings.Join(spoken(m), "\n"); strings.Contains(said, "no answer") {
			t.Errorf("%s: transcript = %q, want no empty-run report", tc.name, said)
		}
	}
}

func TestMultiTurnToolExecutionBoundary(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.cancel = func() {}
	m.conversation = append(m.conversation, nacelle.UserText("read a file"))
	runTwoTurns(m)

	lines := spoken(m)
	if len(lines) < 5 {
		t.Fatalf("spoken lines = %d, want at least 5", len(lines))
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
	line := g.InFlightLine(m.width)
	if len(line) > m.width+len("…") {
		t.Errorf("inFlightLine len = %d, want <= %d", len(line), m.width)
	}
}

func TestTheTokenCountIsASessionTotalThatOnlyEverGrows(t *testing.T) {
	m := sized()
	m.agent = answering(t)

	m.absorb(nacelle.Event{Kind: nacelle.KindTurn, Usage: nacelle.Usage{OutputTokens: 40}})
	m.absorb(nacelle.Event{Kind: nacelle.KindTurn, Usage: nacelle.Usage{OutputTokens: 60}})
	m.absorb(nacelle.Event{Kind: nacelle.KindDone, Usage: nacelle.Usage{OutputTokens: 100}})
	m.settle()
	if status := m.status(); !strings.Contains(status, "out 100") {
		t.Fatalf("status = %q, want 100", status)
	}

	m.prompt.SetValue("again please")
	m.ask()
	defer m.run.cancel()

	m.absorb(nacelle.Event{Kind: nacelle.KindTurn, Usage: nacelle.Usage{OutputTokens: 5}})
	if status := m.status(); !strings.Contains(status, "out 105") {
		t.Errorf("status = %q, want 105", status)
	}

	m.absorb(nacelle.Event{Kind: nacelle.KindDone, Usage: nacelle.Usage{OutputTokens: 7}})
	m.settle()
	if status := m.status(); !strings.Contains(status, "out 107") {
		t.Errorf("status = %q, want 107", status)
	}
}

func TestAStreamErrorIsCommittedAfterTheAnswerItInterrupted(t *testing.T) {
	m := sized()
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "half an answer"})
	m.consume(result{err: errors.New("the stream fell over")})

	screen := onScreen(m)
	answer, failure := strings.Index(screen, "half an answer"), strings.Index(screen, "fell over")
	if answer < 0 || failure < 0 {
		t.Fatalf("screen = %q, want both answer and error", screen)
	}
	if failure < answer {
		t.Errorf("screen = %q, want error under text", screen)
	}
}

func TestASessionThatDidNothingGetsNoRecap(t *testing.T) {
	m := ranFor(20 * time.Minute)
	if got := m.recap(); got != "" {
		t.Errorf("idle session got recap: %q", got)
	}
}

func TestARecapSaysHowLongTheSessionRanAndWhatItSpent(t *testing.T) {
	m := ranFor(14*time.Minute + 3*time.Second)
	m.tools, m.failed = 12, 2
	m.spent = nacelle.Usage{InputTokens: 120000, OutputTokens: 4200, CacheReadTokens: 9800}

	lines := strings.Split(m.recap(), "\n")
	if len(lines) != 2 {
		t.Fatalf("recap is %d lines, want 2: %q", len(lines), m.recap())
	}
	if want := "session · 14m3s · 12 tools · 2 failed"; lines[0] != want {
		t.Errorf("recap line 1 = %q, want %q", lines[0], want)
	}
	if want := "in 120k · out 4.2k · 9.8k cached"; lines[1] != want {
		t.Errorf("recap line 2 = %q, want %q", lines[1], want)
	}
}

func TestARecapLeavesOutTheCountsThatAreZero(t *testing.T) {
	m := ranFor(time.Minute)
	m.spent = nacelle.Usage{InputTokens: 900, OutputTokens: 100}

	line := strings.Split(m.recap(), "\n")[0]
	if strings.Contains(line, "tool") || strings.Contains(line, "failed") {
		t.Errorf("chat with no tool reports tools: %q", line)
	}
	if line != "session · 1m0s" {
		t.Errorf("recap line 1 = %q, want %q", line, "session · 1m0s")
	}
}

func TestARecapCountsASingleCallAsOneTool(t *testing.T) {
	m := ranFor(time.Minute)
	m.tools = 1

	if line := strings.Split(m.recap(), "\n")[0]; !strings.Contains(line, "· 1 tool") || strings.Contains(line, "1 tools") {
		t.Errorf("recap line 1 = %q, want 1 tool", line)
	}
}

func TestARecapShowsACostOnlyWhenTheBackendReportedOne(t *testing.T) {
	m := ranFor(time.Minute)
	m.spent = nacelle.Usage{InputTokens: 2600, OutputTokens: 1100}

	if got := m.recap(); strings.Contains(got, "$") {
		t.Errorf("recap invented a cost: %q", got)
	}

	m.spent.Cost = 0.4821
	if got := m.recap(); !strings.Contains(got, "$0.4821") {
		t.Errorf("recap %q dropped cost", got)
	}
}

func TestARecapCountsTheRunAbandonedOnTheWayOut(t *testing.T) {
	m := ranFor(time.Minute)
	m.spent = nacelle.Usage{InputTokens: 1000}
	m.run.usage = nacelle.Usage{InputTokens: 1000, OutputTokens: 3000}

	if got := m.recap(); !strings.Contains(got, "in 2.0k") || !strings.Contains(got, "out 3.0k") {
		t.Errorf("recap %q forgot run in flight", got)
	}
}

func TestASessionShorterThanASecondIsNotReportedAsInstant(t *testing.T) {
	m := ranFor(200 * time.Millisecond)
	m.tools = 1

	if got := m.recap(); !strings.Contains(got, "session · 1s") {
		t.Errorf("recap %q rounded short session to zero", got)
	}
}
