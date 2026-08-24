package main

import (
	"context"
	"encoding/json"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// The plan tool draws a checklist and does nothing else, and the model is told
// to re-send the whole plan every time a step changes. Asking about it would be
// one keypress per step of every plan, which is how -approve-tools ends up
// switched off — so the gate has to answer for this one itself.
func TestThePlanToolIsNeverPutInFrontOfAHuman(t *testing.T) {
	gate := newApprovals()
	gate.send = func(msg tea.Msg) {
		t.Errorf("the plan tool was put in front of a human: %#v", msg)
		if req, ok := msg.(approvalRequest); ok {
			req.decision <- denied
		}
	}

	input := json.RawMessage(`{"tasks":[{"title":"read the file","status":"in_progress"}]}`)
	if !gate.ask(context.Background(), tasksTool{}.Name(), input) {
		t.Fatal("the plan tool was refused")
	}
}

// The exemption sits behind the duplicate-key refusal on purpose. That check is
// about input nobody can read, it costs nothing to keep, and a plan whose JSON
// says two different things is exactly as unrenderable as any other call's.
func TestThePlanToolIsStillRefusedWhenItsInputIsAmbiguous(t *testing.T) {
	gate := newApprovals()
	gate.send = func(tea.Msg) {
		t.Fatal("an unrenderable call was put in front of a human anyway")
	}

	input := json.RawMessage(`{"tasks":[],"tasks":[{"title":"x","status":"pending"}]}`)
	if gate.ask(context.Background(), tasksTool{}.Name(), input) {
		t.Error("a plan whose input has two values for one key was approved")
	}
}
