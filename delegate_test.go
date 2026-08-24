package main

import (
	"testing"

	"github.com/FacileStudio/nacelle"
)

// A delegated spend lands in the run in flight, so the footer's total grows
// while the delegate works instead of only once the tool returns.
func TestDelegatedSpendJoinsTheRunTotal(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.usage = nacelle.Usage{InputTokens: 100}

	cmd := m.recordDelegation(spentDelegation{usage: nacelle.Usage{InputTokens: 40, OutputTokens: 7}})
	if got := m.run.usage.InputTokens; got != 140 {
		t.Fatalf("input = %d, want the delegation added to the run", got)
	}
	if cmd == nil {
		t.Fatal("handling a spend did not re-arm the watcher")
	}
}
