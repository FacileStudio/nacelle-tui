package main

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Non-interactive approval must still catch duplicate keys — they are
// dangerous whatever the -approve-tools setting.
func TestAcceptRefusesDuplicateKeys(t *testing.T) {
	a := newApprovals()
	input := []byte(`{"command":"ls","command":"rm -rf /"}`)
	if a.accept(context.Background(), "run_command", input) {
		t.Error("a call with duplicate keys was allowed through the non-interactive gate")
	}
}

// Non-interactive approval must let everything else through — the whole
// point is that tools run unasked for everyone who never turns the gate on.
func TestAcceptPassesLegibleInput(t *testing.T) {
	for _, input := range []string{"", "null", "{}", `{"path":"view.go"}`, `[1,2]`} {
		a := newApprovals()
		if !a.accept(context.Background(), "read_file", []byte(input)) {
			t.Errorf("input %q was refused when it should be allowed", input)
		}
	}
}

// Non-interactive approval must not prompt — it has nowhere to send a
// question even if it wanted to, and send is nil when the gate is off.
func TestAcceptDoesNotCallSend(t *testing.T) {
	a := newApprovals()
	if !a.accept(context.Background(), "search", nil) {
		t.Error("a nil-input call was refused")
	}
}

// The one that justifies the whole change: encoding/json keeps the last of
// two identical keys, so the status line renders ls while the tool would run
// rm -rf /. Nobody gets asked a question they cannot read the answer to.
func TestAmbiguousInputIsRefusedWithoutAsking(t *testing.T) {
	asked := false
	a := newApprovals()
	a.send = func(msg tea.Msg) {
		asked = true
		msg.(approvalRequest).decision <- allowedForSession
	}

	input := []byte(`{"command":"ls","command":"rm -rf /"}`)
	if a.ask(context.Background(), "run_command", input) {
		t.Error("a call whose input has two values for one key was approved")
	}
	if asked {
		t.Error("an unrenderable call was put in front of a human anyway")
	}
}

// Allow-for-session is permission for a tool, not a waiver on every input it
// is handed afterwards — so the check has to sit ahead of the allow-list, not
// behind it.
func TestAmbiguousInputIsRefusedEvenForAnAllowedTool(t *testing.T) {
	a := newApprovals()
	a.send = func(msg tea.Msg) {
		msg.(approvalRequest).decision <- allowedForSession
	}
	if !a.ask(context.Background(), "run_command", []byte(`{"command":"ls"}`)) {
		t.Fatal("a legible call was refused")
	}

	if a.ask(context.Background(), "run_command", []byte(`{"command":"ls","command":"rm -rf /"}`)) {
		t.Error("allowing the tool for the session also allowed input nobody can read")
	}
}

// The gate must only refuse ambiguity. A tool with no arguments arrives as
// nil, as null or as {}, and refusing those would turn -approve-tools into a
// switch that denies half the toolbox outright.
func TestNoArgumentCallsStillReachTheHuman(t *testing.T) {
	for _, input := range []string{"", "null", "{}", "[1,2]"} {
		asked := false
		a := newApprovals()
		a.send = func(msg tea.Msg) {
			asked = true
			msg.(approvalRequest).decision <- allowedOnce
		}

		if !a.ask(context.Background(), "list_files", []byte(input)) {
			t.Errorf("input %q was refused outright", input)
		}
		if !asked {
			t.Errorf("input %q never reached the prompt", input)
		}
	}
}
