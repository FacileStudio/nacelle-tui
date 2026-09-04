package status

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
)

func TestShortTokens(t *testing.T) {
	cases := map[int64]string{
		0:         "0",
		999:       "999",
		1000:      "1.0k",
		12345:     "12.3k",
		99999:     "100.0k",
		100000:    "100k",
		123456789: "123M",
		1000000:   "1.0M",
		1234567:   "1.2M",
		99999999:  "100.0M",
	}
	for in, want := range cases {
		if got := ShortTokens(in); got != want {
			t.Errorf("ShortTokens(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestWaitingVerb(t *testing.T) {
	first := WaitingVerb(0)
	if !strings.HasPrefix(first, "waiting ") {
		t.Errorf("WaitingVerb(0) = %q, want prefix 'waiting '", first)
	}
	next := WaitingVerb(Rephrase)
	if next == first {
		t.Errorf("WaitingVerb(%v) = %q, want different from %q", Rephrase, next, first)
	}
	cycle := WaitingVerb(time.Duration(len(WaitingPhrases())) * Rephrase)
	if cycle != first {
		t.Errorf("WaitingVerb after full cycle = %q, want %q", cycle, first)
	}
}

func TestLasted(t *testing.T) {
	if got := Lasted(0); got != "1s" {
		t.Errorf("Lasted(0) = %q, want floor of 1s", got)
	}
	if got := Lasted(45 * time.Second); got != "45s" {
		t.Errorf("Lasted(45s) = %q, want 45s", got)
	}
	if got := Lasted(90 * time.Second); got != "1m30s" {
		t.Errorf("Lasted(90s) = %q, want 1m30s", got)
	}
}

func TestSpinnerKeepsTickingWhileBusy(t *testing.T) {
	s := NewSpinner()
	msg, ok := s.Tick().(spinner.TickMsg)
	if !ok {
		t.Fatal("Tick did not produce a spinner.TickMsg")
	}
	if cmd := s.Spun(msg, true); cmd == nil {
		t.Fatal("Spun returned nil cmd while busy")
	}
}

func TestSpinnerStopsTickingWhenIdle(t *testing.T) {
	s := NewSpinner()
	msg, ok := s.Tick().(spinner.TickMsg)
	if !ok {
		t.Fatal("Tick did not produce a spinner.TickMsg")
	}
	if cmd := s.Spun(msg, false); cmd != nil {
		t.Fatal("Spun returned non-nil cmd when idle")
	}
}
