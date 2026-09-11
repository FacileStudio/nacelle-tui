package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

func TestStreamedOutputRowsCarryNoLeadingMargin(t *testing.T) {
	m := sized()
	m.run.busy = true
	tool := &nacelle.ToolEvent{ID: "1", Name: "run_command", Input: `{"command":"ls"}`}
	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: tool})
	m.absorb(nacelle.Event{Kind: nacelle.KindToolOutput, Tool: tool, Text: "file one"})
	rows := m.aboveContent()
	found := false
	for _, row := range rows {
		for line := range strings.SplitSeq(row, "\n") {
			if !strings.Contains(visible(line), "file one") {
				continue
			}
			found = true
			if got := visible(line); !strings.HasPrefix(got, "▌") {
				t.Errorf("streamed box row = %q, want it to start with the border glyph", got)
			}
		}
	}
	if !found {
		t.Fatalf("no streamed output row in\n%s", strings.Join(rows, "\n"))
	}
	_ = time.Now
}
