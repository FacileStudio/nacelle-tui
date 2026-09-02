package main

import (
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

// ranFor is a model that has been open for a fixed stretch, so a test can
// assert on the duration a recap prints without sleeping through it.
func ranFor(spent time.Duration) *model {
	m := bareBanner()
	m.began = time.Now().Add(-spent)
	return m
}

func TestASessionThatDidNothingGetsNoRecap(t *testing.T) {
	m := ranFor(20 * time.Minute)

	if got := m.recap(); got != "" {
		t.Errorf("an idle session got a recap: %q", got)
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
		t.Errorf("a chat that ran no tool reports tools: %q", line)
	}
	if line != "session · 1m0s" {
		t.Errorf("recap line 1 = %q, want %q", line, "session · 1m0s")
	}
}

func TestARecapCountsASingleCallAsOneTool(t *testing.T) {
	m := ranFor(time.Minute)
	m.tools = 1

	if line := strings.Split(m.recap(), "\n")[0]; !strings.Contains(line, "· 1 tool") || strings.Contains(line, "1 tools") {
		t.Errorf("recap line 1 = %q, want a singular tool", line)
	}
}

func TestARecapShowsACostOnlyWhenTheBackendReportedOne(t *testing.T) {
	m := ranFor(time.Minute)
	m.spent = nacelle.Usage{InputTokens: 2600, OutputTokens: 1100}

	if got := m.recap(); strings.Contains(got, "$") {
		t.Errorf("recap invented a cost for a backend that reported none: %q", got)
	}

	m.spent.Cost = 0.4821
	if got := m.recap(); !strings.Contains(got, "$0.4821") {
		t.Errorf("recap %q dropped the cost the backend reported", got)
	}
}

func TestARecapCountsTheRunAbandonedOnTheWayOut(t *testing.T) {
	m := ranFor(time.Minute)
	m.spent = nacelle.Usage{InputTokens: 1000}
	m.run.usage = nacelle.Usage{InputTokens: 1000, OutputTokens: 3000}

	if got := m.recap(); !strings.Contains(got, "in 2.0k") || !strings.Contains(got, "out 3.0k") {
		t.Errorf("recap %q forgot the run still in flight", got)
	}
}

func TestASessionShorterThanASecondIsNotReportedAsInstant(t *testing.T) {
	m := ranFor(200 * time.Millisecond)
	m.tools = 1

	if got := m.recap(); !strings.Contains(got, "session · 1s") {
		t.Errorf("recap %q rounded a short session to zero", got)
	}
}
