package main

import (
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

// compactAt is the session default, kept as a literal here rather than
// imported from the internal settings package, which package main cannot
// import. It must stay in step with settings.DefaultCompactAt: if that
// moves, this and the 100_000 in sized()/bareBanner move with it by hand.
const compactAt int64 = 100_000

func bigConversation() []nacelle.Message {
	result := func(id string, n int) nacelle.ToolResult {
		return nacelle.ToolResult{ID: id, Name: "read", Result: strings.Repeat("x", n)}
	}
	return []nacelle.Message{
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{result("old-1", 40_000), result("small", 100)}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "earlier answer"}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{result("old-2", 30_000)}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "middle answer"}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{result("recent", 50_000)}},
		{Role: nacelle.RoleAssistant},
	}
}

func TestCompactDropsOldLargeResults(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000

	m.compact()

	dropped, kept := 0, 0
	for _, message := range m.conversation {
		for _, part := range message.Parts {
			result, ok := part.(nacelle.ToolResult)
			if !ok {
				continue
			}
			if strings.HasPrefix(result.Result, droppedNotice) {
				dropped++
				continue
			}
			if len(result.Result) > compactMinResult {
				kept++
			}
		}
	}
	if dropped != 1 || kept != 2 {
		t.Errorf("dropped %d and kept %d large results, want the one outside the keep window dropped and both inside it kept", dropped, kept)
	}
	if m.trimmed != 1 {
		t.Errorf("trimmed count = %d, want 1", m.trimmed)
	}
}

func TestCompactKeepsThePairingShape(t *testing.T) {
	m := sized()
	before := bigConversation()
	m.conversation = before
	m.size = compactAt + 1

	m.compact()

	for i, message := range m.conversation {
		if len(message.Parts) != len(before[i].Parts) {
			t.Fatalf("message %d changed shape: %d parts, was %d", i, len(message.Parts), len(before[i].Parts))
		}
		for j, part := range message.Parts {
			was, ok := before[i].Parts[j].(nacelle.ToolResult)
			now, still := part.(nacelle.ToolResult)
			if ok != still {
				t.Fatalf("message %d part %d changed kind", i, j)
			}
			if ok && now.ID != was.ID {
				t.Errorf("message %d part %d: id %q, was %q — the pairing broke", i, j, now.ID, was.ID)
			}
		}
	}
}

func TestCompactLeavesAShortConversationAlone(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 1
	m.conversation = m.conversation[len(m.conversation)-compactKeepMessages:]

	m.compact()

	if m.trimmed != 0 {
		t.Errorf("a conversation inside the keep window lost %d results", m.trimmed)
	}
}

func TestCompactDoesNothingUnderTheThreshold(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt

	m.compact()

	if m.trimmed != 0 {
		t.Errorf("at the threshold exactly, trimmed %d anyway", m.trimmed)
	}
}

func TestCompactIsNotPaidTwiceOnASecondPass(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 1

	m.compact()
	first := m.trimmed
	m.size = compactAt + compactSlack + 1

	m.compact()

	if m.trimmed != first {
		t.Errorf("second pass trimmed %d more; placeholders were counted as savings again", m.trimmed-first)
	}
}

func TestCompactDropsOldThinkingBlocks(t *testing.T) {
	msg := func(role nacelle.Role, parts ...nacelle.Part) nacelle.Message {
		return nacelle.Message{Role: role, Parts: parts}
	}
	thought := func(n int) nacelle.Reasoning {
		return nacelle.Reasoning{Text: strings.Repeat("thinking about this ", n)}
	}
	m := sized()
	m.conversation = []nacelle.Message{
		msg(nacelle.RoleUser, nacelle.ToolResult{ID: "a", Name: "read", Result: strings.Repeat("x", 2000)}),
		msg(nacelle.RoleAssistant, thought(5000), nacelle.Text{Text: "old conclusion"}),
		msg(nacelle.RoleUser, nacelle.ToolResult{ID: "b", Name: "read", Result: strings.Repeat("x", 2000)}),
		msg(nacelle.RoleAssistant, thought(3000), nacelle.Text{Text: "middle conclusion"}),
		msg(nacelle.RoleUser, nacelle.ToolResult{ID: "c", Name: "read", Result: strings.Repeat("x", 2000)}),
		msg(nacelle.RoleAssistant, thought(100), nacelle.Text{Text: "recent conclusion"}),
	}
	m.size = compactAt + 50_000

	m.compact()

	// Only the first assistant message (index 1) is outside the keep window.
	// Index 3 is among the last 4 messages, so its thinking is preserved.
	replaced, untouched := 0, 0
	for _, message := range m.conversation {
		for _, part := range message.Parts {
			r, ok := part.(nacelle.Reasoning)
			if !ok {
				continue
			}
			if strings.HasPrefix(r.Text, droppedThinkingNotice) {
				replaced++
			} else {
				untouched++
			}
		}
	}
	if replaced != 1 {
		t.Errorf("%d thinking blocks replaced, want 1 (the one outside the keep window)", replaced)
	}
	if untouched != 2 {
		t.Errorf("%d thinking blocks untouched, want 2 (the two inside the keep window)", untouched)
	}
	// The assistant text parts should still be there.
	for i := 1; i < len(m.conversation); i += 2 {
		hasText := false
		for _, part := range m.conversation[i].Parts {
			if _, ok := part.(nacelle.Text); ok {
				hasText = true
				break
			}
		}
		if !hasText {
			t.Errorf("assistant message %d lost its Text part after compact", i)
		}
	}
	if m.trimmed < 2 {
		t.Errorf("trimmed count = %d, want at least 2 (1 tool result + 1 thinking block)", m.trimmed)
	}
}
func TestSizedCountsEveryBilledInputKind(t *testing.T) {
	m := sized()
	m.sized(nacelle.Usage{InputTokens: 1000, CacheReadTokens: 9000, CacheCreationTokens: 500})
	if m.size != 10_500 {
		t.Errorf("size = %d, want cache reads and creations billed as input", m.size)
	}
}
