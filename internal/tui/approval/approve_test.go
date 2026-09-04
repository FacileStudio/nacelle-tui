package approval

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestAcceptRefusesDuplicateKeys(t *testing.T) {
	a := New()
	input := []byte(`{"command":"ls","command":"rm -rf /"}`)
	if a.Accept(context.Background(), "run_command", input) {
		t.Error("a call with duplicate keys was allowed through the non-interactive gate")
	}
}

func TestAcceptPassesLegibleInput(t *testing.T) {
	for _, input := range []string{"", "null", "{}", `{"path":"view.go"}`, `[1,2]`} {
		a := New()
		if !a.Accept(context.Background(), "read_file", []byte(input)) {
			t.Errorf("input %q was refused when it should be allowed", input)
		}
	}
}

func TestAcceptDoesNotCallSend(t *testing.T) {
	a := New()
	if !a.Accept(context.Background(), "search", nil) {
		t.Error("a nil-input call was refused")
	}
}

func TestAmbiguousInputIsRefusedWithoutAsking(t *testing.T) {
	asked := false
	a := New()
	a.send = func(msg tea.Msg) {
		asked = true
		msg.(Request).Decision <- AllowedForSession
	}

	input := []byte(`{"command":"ls","command":"rm -rf /"}`)
	if a.Ask(context.Background(), "run_command", input) {
		t.Error("a call whose input has two values for one key was approved")
	}
	if asked {
		t.Error("an unrenderable call was put in front of a human anyway")
	}
}

func TestAmbiguousInputIsRefusedEvenForAnAllowedTool(t *testing.T) {
	a := New()
	a.send = func(msg tea.Msg) {
		msg.(Request).Decision <- AllowedForSession
	}
	if !a.Ask(context.Background(), "run_command", []byte(`{"command":"ls"}`)) {
		t.Fatal("a legible call was refused")
	}

	if a.Ask(context.Background(), "run_command", []byte(`{"command":"ls","command":"rm -rf /"}`)) {
		t.Error("allowing the tool for the session also allowed input nobody can read")
	}
}

func TestNoArgumentCallsStillReachTheHuman(t *testing.T) {
	for _, input := range []string{"", "null", "{}", "[1,2]"} {
		asked := false
		a := New()
		a.send = func(msg tea.Msg) {
			asked = true
			msg.(Request).Decision <- AllowedOnce
		}

		if !a.Ask(context.Background(), "list_files", []byte(input)) {
			t.Errorf("input %q was refused outright", input)
		}
		if !asked {
			t.Errorf("input %q never reached the prompt", input)
		}
	}
}
