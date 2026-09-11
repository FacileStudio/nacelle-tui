package tui

import (
	"encoding/json"
	"testing"

	"github.com/FacileStudio/nacelle"
)

// TestAlignedEvictCutNeverSplitsAToolPair covers the one boundary that breaks
// a pairing: a cut landing right after an assistant ToolCall message, so the
// kept tail opens with the user ToolResult answering an evicted call id, which
// Anthropic rejects. The helper must pull the cut back one message when the
// first kept turn answers the last evicted one, and leave any other cut
// untouched.
func TestAlignedEvictCutNeverSplitsAToolPair(t *testing.T) {
	conv := []nacelle.Message{
		nacelle.UserText("setup"),
		nacelle.AssistantText("intro"),
		nacelle.UserText("run the tool"),
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{
			nacelle.ToolCall{ID: "call_1", Name: "read", Input: json.RawMessage(`{}`), Finished: true},
		}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{
			nacelle.ToolResult{ID: "call_1", Name: "read", Result: "file contents"},
		}},
		nacelle.UserText("keep going"),
		nacelle.AssistantText("done"),
	}

	if got := alignedEvictCut(conv, 4); got != 3 {
		t.Errorf("alignedEvictCut = %d, want 3 (kept tail must not open with a tool_result)", got)
	}
	if got := alignedEvictCut(conv, 3); got != 3 {
		t.Errorf("alignedEvictCut = %d, want 3", got)
	}
	if got := alignedEvictCut(conv, 5); got != 5 {
		t.Errorf("alignedEvictCut = %d, want 5", got)
	}
	if got := alignedEvictCut(conv, 2); got != 2 {
		t.Errorf("alignedEvictCut = %d, want 2", got)
	}
}
