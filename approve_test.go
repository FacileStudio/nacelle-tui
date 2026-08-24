package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Nothing calls ask before -approve-tools is on, and that flag stays off by
// default — but this proves the one behaviour everything else depends on:
// a decision actually reaches the caller waiting on it.
func TestApprovalReachesTheCaller(t *testing.T) {
	a := newApprovals()
	a.send = func(msg tea.Msg) {
		req, ok := msg.(approvalRequest)
		if !ok {
			t.Fatalf("sent %T, want approvalRequest", msg)
		}
		req.decision <- allowedOnce
	}

	if !a.ask(context.Background(), "search", nil) {
		t.Fatal("an allowed call was reported as refused")
	}
}

func TestADenialIsReportedAsRefused(t *testing.T) {
	a := newApprovals()
	a.send = func(msg tea.Msg) {
		msg.(approvalRequest).decision <- denied
	}

	if a.ask(context.Background(), "search", nil) {
		t.Fatal("a denied call was reported as allowed")
	}
}

// The whole point of "allow for the rest of this session": asking once
// should be the last time, not the first of many.
func TestAllowedForSessionIsNotAskedAgain(t *testing.T) {
	var calls int
	a := newApprovals()
	a.send = func(msg tea.Msg) {
		calls++
		msg.(approvalRequest).decision <- allowedForSession
	}

	for i := 0; i < 3; i++ {
		if !a.ask(context.Background(), "search", nil) {
			t.Fatalf("call %d was refused", i)
		}
	}
	if calls != 1 {
		t.Errorf("asked %d times, want 1 — allowed-for-session must not ask again", calls)
	}
}

// The opposite has to hold too, or a single "no" would read as "no,
// forever" the same way "yes" can mean "yes, forever".
func TestADenialIsAskedAgainNextTime(t *testing.T) {
	var calls int
	a := newApprovals()
	a.send = func(msg tea.Msg) {
		calls++
		msg.(approvalRequest).decision <- denied
	}

	a.ask(context.Background(), "search", nil)
	a.ask(context.Background(), "search", nil)
	if calls != 2 {
		t.Errorf("asked %d times, want 2 — a denial must not be remembered the way allow-for-session is", calls)
	}
}

// Allowing one tool for the session must not silently allow every other
// tool too — the whole point of naming which tool in the prompt.
func TestAllowingOneToolDoesNotAllowAnother(t *testing.T) {
	a := newApprovals()
	a.send = func(msg tea.Msg) {
		msg.(approvalRequest).decision <- allowedForSession
	}
	a.ask(context.Background(), "search", nil)

	asked := false
	a.send = func(msg tea.Msg) {
		asked = true
		msg.(approvalRequest).decision <- allowedOnce
	}
	a.ask(context.Background(), "write_file", nil)

	if !asked {
		t.Error("a different tool was allowed without ever being asked")
	}
}

// This is the one that matters most: a human who never answers must not
// hang the whole client. Cancelling the run's context is the existing
// escape hatch for a wedged tool, and it has to unblock a wedged approval
// the same way, or -approve-tools would trade one kind of stuck for another.
//
// send never answers in this test — that is the whole point, a decision
// that never arrives — and the sleep before cancel gives the goroutine time
// to actually reach the select before the context it is waiting on dies.
func TestAskUnblocksWhenItsContextIsCancelled(t *testing.T) {
	a := newApprovals()
	a.send = func(tea.Msg) {}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() { done <- a.ask(ctx, "search", nil) }()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case allowed := <-done:
		if allowed {
			t.Error("a cancelled wait was reported as approved")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ask did not unblock when its context was cancelled — this would hang the whole TUI")
	}
}

// The SDK runs tool calls concurrently within a turn (confirmed against the
// vendor SDK's own doc comment: "tool handlers ARE called concurrently when
// multiple tools are invoked in a single turn"), and this model has exactly
// one place to show a question. Two decisions in flight at once would mean
// the second overwrites the first's prompt before anyone could answer it.
//
// Each simulated answer sleeps briefly before deciding, standing in for a
// human taking a moment to read the question — long enough that a second,
// unserialized ask would overlap it if the lock were not there.
func TestConcurrentAsksAreSerializedToOneAtATime(t *testing.T) {
	var inFlight int32
	var maxObserved int32
	a := newApprovals()
	a.send = func(msg tea.Msg) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&maxObserved)
			if n <= old || atomic.CompareAndSwapInt32(&maxObserved, old, n) {
				break
			}
		}
		req := msg.(approvalRequest)
		go func() {
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			req.decision <- allowedOnce
		}()
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			a.ask(context.Background(), fmt.Sprintf("tool-%d", n), nil)
		}(i)
	}
	wg.Wait()

	if maxObserved > 1 {
		t.Errorf("max concurrent prompts = %d, want at most 1 — approvals must serialize", maxObserved)
	}
}

// truncate is what keeps a tool call with a large argument list from
// running off the end of the status line, which is also the reason that line
// shows the input bytes rather than the decoded call — and the reason a
// repeated key has to be stopped before it ever reaches it.

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
