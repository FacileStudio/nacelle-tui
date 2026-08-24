package main

import (
	"context"
	"encoding/json"
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

// Handing the SDK a nil Approve for the nested run means deny-all, so the
// delegate would be given every tool and refused every call. This fails if
// the explicit allow-all is ever dropped back to nil.
func TestADelegateWithNoApprovalGateMayCall(t *testing.T) {
	policy := delegateApprovals(nil)
	if policy == nil {
		t.Fatal("no policy was built, which the SDK reads as deny-all")
	}
	if !policy(context.Background(), "read_file", json.RawMessage(`{}`)) {
		t.Error("a call was refused in a session that asks about nothing")
	}
}

// The parent's gate is the parent's answer, and a delegate does not get to
// route around it. This fails if the nil case ever grows into an
// unconditional allow-all that swallows a real gate.
func TestADelegateStillObeysTheParentsGate(t *testing.T) {
	asked := ""
	gate := nacelle.Approve(func(_ context.Context, name string, _ json.RawMessage) bool {
		asked = name
		return name == "read_file"
	})

	policy := delegateApprovals(gate)
	if !policy(context.Background(), "read_file", json.RawMessage(`{}`)) {
		t.Error("the gate allowed read_file and the delegate was refused anyway")
	}
	if policy(context.Background(), "run_command", json.RawMessage(`{}`)) {
		t.Error("the gate refused run_command and the delegate ran it")
	}
	if asked != "run_command" {
		t.Errorf("the gate was asked about %q, want the delegate's own calls", asked)
	}
}
