package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

func TestMaskFloorDropsOldLargeResults(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000
	evictCut := len(m.conversation) - keepCount(len(m.conversation))

	m.maskEvicted(evictCut)

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

func TestMaskKeepsThePairingShape(t *testing.T) {
	m := sized()
	before := bigConversation()
	m.conversation = before
	m.size = compactAt + 1
	evictCut := len(m.conversation) - keepCount(len(m.conversation))

	m.maskEvicted(evictCut)

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

func TestMaskLeavesAShortConversationAlone(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 1
	m.conversation = m.conversation[len(m.conversation)-keepCount(len(m.conversation)):]
	m.maskEvicted(0)

	if m.trimmed != 0 {
		t.Errorf("a conversation with an empty eviction lost %d results", m.trimmed)
	}
}

func TestMaskIsNotPaidTwiceOnASecondPass(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 1
	evictCut := len(m.conversation) - keepCount(len(m.conversation))

	m.maskEvicted(evictCut)
	first := m.trimmed
	m.size = compactAt + compactSlack + 1

	m.maskEvicted(evictCut)

	if m.trimmed != first {
		t.Errorf("second pass trimmed %d more; placeholders were counted as savings again", m.trimmed-first)
	}
}

func TestMaskDropsOldThinkingBlocks(t *testing.T) {
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
	evictCut := len(m.conversation) - keepCount(len(m.conversation))

	m.maskEvicted(evictCut)

	replaced, untouched := countThinkingBlocks(m.conversation)
	if replaced != 1 {
		t.Errorf("%d thinking blocks replaced, want 1", replaced)
	}
	if untouched != 2 {
		t.Errorf("%d thinking blocks untouched, want 2", untouched)
	}
	verifyAssistantTextPreserved(t, m.conversation)
	if m.trimmed < 2 {
		t.Errorf("trimmed count = %d, want at least 2", m.trimmed)
	}
}

func TestMaskFallbackKeepsTheConversationStanding(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000
	m.compactAt = compactAt

	outcome := compactOutcome{before: m.size, evictCut: len(m.conversation) - keepCount(len(m.conversation))}
	m.applyMaskFallback(outcome)

	saved := 0
	for _, message := range m.conversation {
		for _, part := range message.Parts {
			result, ok := part.(nacelle.ToolResult)
			if ok && strings.HasPrefix(result.Result, droppedNotice) {
				saved++
			}
		}
	}
	if saved == 0 {
		t.Errorf("mask fallback masked nothing")
	}
	if len(m.conversation) != len(bigConversation()) {
		t.Errorf("mask fallback changed the message count: %d, was %d", len(m.conversation), len(bigConversation()))
	}
	said := spoken(m)
	if len(said) < 1 || !strings.Contains(strings.Join(said, " "), "✂ compacted context") {
		t.Errorf("mask fallback did not report: %v", said)
	}
}

func TestSettleCompactionFallsBackToTheMaskOnFailure(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000
	outcome := compactOutcome{before: m.size, evictCut: len(m.conversation) - keepCount(len(m.conversation)), err: errors.New("summarizer hiccuped")}

	m.settleCompaction(outcome)

	if m.compacting {
		t.Errorf("compacting still true after the fallback")
	}
	if len(m.conversation) != len(bigConversation()) {
		t.Errorf("mask fallback changed the message count: %d", len(m.conversation))
	}
	saved := 0
	for _, message := range m.conversation {
		for _, part := range message.Parts {
			result, ok := part.(nacelle.ToolResult)
			if ok && strings.HasPrefix(result.Result, droppedNotice) {
				saved++
			}
		}
	}
	if saved == 0 {
		t.Errorf("a failed summary bought no headroom: nothing was masked")
	}
	said := strings.Join(spoken(m), " ")
	if !strings.Contains(said, "compaction summary failed") {
		t.Errorf("failure not reported: %v", said)
	}
}

func TestSizedCountsEveryBilledInputKind(t *testing.T) {
	m := sized()
	m.sized(nacelle.Usage{InputTokens: 1000, CacheReadTokens: 9000, CacheCreationTokens: 500})
	if m.size != 10_500 {
		t.Errorf("size = %d, want cache reads and creations billed as input", m.size)
	}
}

func TestCompactedHistoryCarriesTheMarker(t *testing.T) {
	block := compactedHistory("Decisions:\n- ship it")
	text, ok := block.Parts[0].(nacelle.Text)
	if !ok || !strings.HasPrefix(text.Text, "[compacted context") {
		t.Errorf("compacted history = %q, want the marker prefix", text.Text)
	}
}

func countThinkingBlocks(messages []nacelle.Message) (int, int) {
	replaced, untouched := 0, 0
	for _, msg := range messages {
		r, u := countMessageThinking(msg)
		replaced += r
		untouched += u
	}
	return replaced, untouched
}

func countMessageThinking(msg nacelle.Message) (int, int) {
	replaced, untouched := 0, 0
	for _, part := range msg.Parts {
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
	return replaced, untouched
}

func verifyAssistantTextPreserved(t *testing.T, messages []nacelle.Message) {
	t.Helper()
	for i := 1; i < len(messages); i += 2 {
		found := false
		for _, part := range messages[i].Parts {
			if _, ok := part.(nacelle.Text); ok {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("assistant message %d lost its Text part after compact", i)
		}
	}
}
