package main

import (
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

// The turn does not end when the thinking does. flush runs at the end of the
// turn — after the answer has streamed, and after any tool the model called
// has run — so a clock measured to it reports the whole turn as thinking.
// Measured before this was fixed: 0.6s of reasoning followed by a 0.6s answer
// printed "thought for 1.2s".
func TestTheClockStopsWhenTheAnswerStartsNotWhenTheTurnEnds(t *testing.T) {
	m := newModel(nil, "banner", nil)
	m.width = 80

	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: "pondering"})
	m.stamp()
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "the answer"})
	thinking := m.elapsed()

	time.Sleep(50 * time.Millisecond)
	if after := m.elapsed(); after != thinking {
		t.Errorf("the clock kept running after the answer began: %s then %s", thinking, after)
	}
}

// A tool call ends the thinking too, and it is the case that matters most: the
// tool runs between the call and its result, and the turn is not committed
// until the result arrives, so a slow tool would be billed to the model's
// thinking in full.
func TestAToolCallStopsTheClockTheSameWay(t *testing.T) {
	m := newModel(nil, "banner", nil)
	m.width = 80

	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: "which tool"})
	m.stamp()
	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: "1", Name: "read_file", Input: `{"path":"view.go"}`}})
	thinking := m.elapsed()

	time.Sleep(50 * time.Millisecond)
	if after := m.elapsed(); after != thinking {
		t.Errorf("the tool's own runtime is being billed to thinking: %s then %s", thinking, after)
	}
}

// A cleared session is one somebody wanted gone. A key that reprints what the
// model was thinking just before they cleared is that session coming back.
func TestClearingTheSessionDropsWhatCtrlTWouldReprint(t *testing.T) {
	m := newModel(nil, "banner", nil)
	m.absorb(nacelle.Event{Kind: nacelle.KindThinking, Text: "from the old session"})
	m.flush()

	m.clear()
	m.unprinted = nil
	m.reveal()

	for _, line := range m.unprinted {
		if strings.Contains(line, "from the old session") {
			t.Errorf("ctrl+t after /clear reprinted the cleared session: %q", line)
		}
	}
}
