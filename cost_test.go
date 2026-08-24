package main

import (
	"strings"
	"testing"

	"time"

	"github.com/FacileStudio/nacelle"
)

// /cost reads the same accounting footer and recap do, so it has to agree
// with the sum they both print — spent plus the run in flight.
func TestSlashCostReportsTheSessionTotal(t *testing.T) {
	m := sized()
	m.spent = nacelle.Usage{InputTokens: 400, OutputTokens: 100, Cost: 0.01}
	m.run.usage = nacelle.Usage{InputTokens: 600, OutputTokens: 900,
		CacheReadTokens: 2000, Cost: 0.02}
	m.tools, m.failed = 3, 1

	m.prompt.SetValue("/cost")
	said := printedBy(m.ask())
	if !strings.Contains(said, "in 1.0k") || !strings.Contains(said, "out 1.0k") {
		t.Errorf("said = %q, want the merged token totals", said)
	}
	if !strings.Contains(said, "2.0k cached") {
		t.Errorf("said = %q, want the cache reads", said)
	}
	if !strings.Contains(said, "$0.0300") {
		t.Errorf("said = %q, want spent plus the run in flight", said)
	}
	if !strings.Contains(said, "3 tools · 1 failed") {
		t.Errorf("said = %q, want the tool counts", said)
	}
}

// A session that has done nothing says so rather than printing zeroes.
func TestSlashCostOnAnEmptySession(t *testing.T) {
	m := sized()

	m.prompt.SetValue("/cost")
	said := printedBy(m.ask())
	if strings.Contains(said, "$") || strings.Contains(said, "tool") {
		t.Errorf("said = %q, want no cost and no tools for an unused session", said)
	}
	if !strings.Contains(said, "session · ") {
		t.Errorf("said = %q, want the session line regardless", said)
	}
}

// The duration is measured against when the session began, floored at a
// second like recap's — not against the zero Time a fresh model carries.
func TestSlashCostDurationsFromSessionStart(t *testing.T) {
	m := sized()
	m.began = time.Now().Add(-90 * time.Second)

	m.prompt.SetValue("/cost")
	said := printedBy(m.ask())
	if !strings.Contains(said, "session · 1m30s") {
		t.Errorf("said = %q, want the session's own span", said)
	}
}
