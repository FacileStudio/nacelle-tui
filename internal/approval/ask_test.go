package approval

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestApprovalReachesTheCaller(t *testing.T) {
	a := New()
	a.send = func(msg tea.Msg) {
		req, ok := msg.(Request)
		if !ok {
			t.Fatalf("sent %T, want Request", msg)
		}
		req.Decision <- AllowedOnce
	}

	if !a.Ask(context.Background(), "search", nil) {
		t.Fatal("an allowed call was reported as refused")
	}
}

func TestADenialIsReportedAsRefused(t *testing.T) {
	a := New()
	a.send = func(msg tea.Msg) {
		msg.(Request).Decision <- Denied
	}

	if a.Ask(context.Background(), "search", nil) {
		t.Fatal("a denied call was reported as allowed")
	}
}

func TestAllowedForSessionIsNotAskedAgain(t *testing.T) {
	var calls int
	a := New()
	a.send = func(msg tea.Msg) {
		calls++
		msg.(Request).Decision <- AllowedForSession
	}

	for i := range 3 {
		if !a.Ask(context.Background(), "search", nil) {
			t.Fatalf("call %d was refused", i)
		}
	}
	if calls != 1 {
		t.Errorf("asked %d times, want 1", calls)
	}
}

func TestADenialIsAskedAgainNextTime(t *testing.T) {
	var calls int
	a := New()
	a.send = func(msg tea.Msg) {
		calls++
		msg.(Request).Decision <- Denied
	}

	a.Ask(context.Background(), "search", nil)
	a.Ask(context.Background(), "search", nil)
	if calls != 2 {
		t.Errorf("asked %d times, want 2", calls)
	}
}

func TestAllowingOneToolDoesNotAllowAnother(t *testing.T) {
	a := New()
	a.send = func(msg tea.Msg) {
		msg.(Request).Decision <- AllowedForSession
	}
	a.Ask(context.Background(), "search", nil)

	asked := false
	a.send = func(msg tea.Msg) {
		asked = true
		msg.(Request).Decision <- AllowedOnce
	}
	a.Ask(context.Background(), "write_file", nil)

	if !asked {
		t.Error("a different tool was allowed without ever being asked")
	}
}

func TestAskUnblocksWhenItsContextIsCancelled(t *testing.T) {
	a := New()
	a.send = func(tea.Msg) {}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() { done <- a.Ask(ctx, "search", nil) }()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case allowed := <-done:
		if allowed {
			t.Error("a cancelled wait was reported as approved")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Ask did not unblock when context was cancelled")
	}
}

func TestConcurrentAsksAreSerializedToOneAtATime(t *testing.T) {
	var inFlight atomic.Int32
	var maxObserved atomic.Int32
	a := New()
	a.send = func(msg tea.Msg) {
		n := inFlight.Add(1)
		for {
			old := maxObserved.Load()
			if n <= old || maxObserved.CompareAndSwap(old, n) {
				break
			}
		}
		req := msg.(Request)
		go func() {
			time.Sleep(5 * time.Millisecond)
			inFlight.Add(-1)
			req.Decision <- AllowedOnce
		}()
	}

	var wg sync.WaitGroup
	for i := range 5 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			a.Ask(context.Background(), fmt.Sprintf("tool-%d", n), nil)
		}(i)
	}
	wg.Wait()

	if maxObserved.Load() > 1 {
		t.Errorf("max concurrent prompts = %d, want at most 1", maxObserved.Load())
	}
}
