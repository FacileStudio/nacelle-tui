package tui

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

func TestKeepCountKeepsAFractionOfLongConversations(t *testing.T) {
	t.Run("short conversation is kept whole", func(t *testing.T) {
		if kept := keepCount(3); kept != 3 {
			t.Errorf("keepCount(3) = %d, want 3", kept)
		}
	})
	t.Run("floor applies to a small conversation", func(t *testing.T) {
		if kept := keepCount(6); kept != 4 {
			t.Errorf("keepCount(6) = %d, want the 4-message floor", kept)
		}
	})
	t.Run("a quarter of a long conversation stays verbatim", func(t *testing.T) {
		if kept := keepCount(40); kept != 10 {
			t.Errorf("keepCount(40) = %d, want 10 (25%%)", kept)
		}
		if kept := keepCount(80); kept != 20 {
			t.Errorf("keepCount(80) = %d, want 20 (25%%)", kept)
		}
	})
}

func TestApplyReplacesTheEvictedMiddleWithASummary(t *testing.T) {
	conv := bigConversation()
	evictCut := len(conv) - keepCount(len(conv))

	outcome := compactApply(conv, evictCut, "Decisions:\n- went with the read.", int64(125_000))

	assertSummaryBlock(t, conv, evictCut, outcome)
	assertKeptTail(t, outcome.conversation, conv, evictCut)
	if outcome.done.turns != evictCut {
		t.Errorf("summarized turns = %d, want %d", outcome.done.turns, evictCut)
	}
	if outcome.done.results != 0 {
		t.Errorf("masked results = %d, want 0 on a summarized pass — the mask only runs as a fallback", outcome.done.results)
	}
	if outcome.after >= outcome.before {
		t.Errorf("after = %d, want a pass to shrink before = %d", outcome.after, outcome.before)
	}
	if !strings.Contains(outcome.summary, "Decisions:") {
		t.Errorf("outcome summary = %q, want the summary carried back", outcome.summary)
	}
}

// assertSummaryBlock checks the leading user turn of the rebuilt conversation:
// that it replaced the evicted middle, holds the summary and the compacted
// marker, and kept the merged turn's original tool result.
func assertSummaryBlock(t *testing.T, conv []nacelle.Message, evictCut int, outcome compactOutcome) {
	if len(outcome.conversation) != len(conv)-evictCut {
		t.Errorf("conversation after = %d messages, want the evicted middle replaced by one summary + the kept tail", len(outcome.conversation))
	}
	first := outcome.conversation[0]
	if first.Role != nacelle.RoleUser {
		t.Errorf("summary block role = %q, want user", first.Role)
	}
	text, ok := first.Parts[0].(nacelle.Text)
	if !ok || !strings.Contains(text.Text, "Decisions:") {
		t.Errorf("summary block = %q, want the summary kept inside", text.Text)
	}
	if !strings.HasPrefix(text.Text, "[compacted context") {
		t.Errorf("summary block = %q, want the compacted marker", text.Text)
	}
	lastResult, lastOk := first.Parts[len(first.Parts)-1].(nacelle.ToolResult)
	if !lastOk || lastResult.ID != "old-2" {
		t.Errorf("the merged user turn lost its original tool result: %v", first.Parts)
	}
}

// assertKeptTail checks that the kept tail of the rebuilt conversation matches
// the original messages verbatim, in order.
func assertKeptTail(t *testing.T, conversation []nacelle.Message, conv []nacelle.Message, evictCut int) {
	for i := range len(conversation) {
		if conversation[i].Role != conv[evictCut+i].Role {
			t.Errorf("message %d of the tail does not match the kept message %d", i, evictCut+i)
		}
	}
}

func TestCompactedConversationKeepsRolesAlternating(t *testing.T) {
	alternates := func(messages []nacelle.Message) bool {
		for i := 1; i < len(messages); i++ {
			if messages[i].Role == messages[i-1].Role {
				return false
			}
		}
		return true
	}

	merged := compactedConversation(bigConversation(), 2, "Decisions:\n- done.")
	if !alternates(merged) {
		t.Errorf("conversation = %v, want the summary folded in rather than two user turns in a row", merged)
	}

	noMerge := []nacelle.Message{
		{Role: nacelle.RoleUser},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "old"}}},
		{Role: nacelle.RoleUser},
		{Role: nacelle.RoleAssistant},
	}
	inserted := compactedConversation(noMerge, 1, "Decisions:\n- done.")
	if !alternates(inserted) {
		t.Errorf("conversation = %v, want the summary inserted with alternation preserved", inserted)
	}
	if inserted[0].Role != nacelle.RoleUser || inserted[1].Role != nacelle.RoleAssistant {
		t.Errorf("conversation = %v, want summary-user then the kept assistant, roles alternating", inserted)
	}
}

func TestSummarizerSeesTheRawEvictedChunk(t *testing.T) {
	conv := bigConversation()
	evictCut := len(conv) - keepCount(len(conv))

	prompt := compactPrompt(conv, evictCut)

	for i := range evictCut {
		for _, part := range prompt[i].Parts {
			result, ok := part.(nacelle.ToolResult)
			if !ok {
				continue
			}
			if strings.HasPrefix(result.Result, droppedNotice) {
				t.Errorf("message %d fed the summarizer already masked (%q), want the raw result", i, result.Result)
			}
		}
	}
}

func TestCompactReportNamesTheWholePass(t *testing.T) {
	outcome := compactApply(bigConversation(), 2, "Decisions:\n- done.", int64(125_000))

	line := compactReport(outcome)

	if !strings.Contains(line, "✂") {
		t.Errorf("report = %q, want the compaction icon", line)
	}
	if !strings.Contains(line, "Compaction summary") {
		t.Errorf("report = %q, want the summary heading", line)
	}
	if !strings.Contains(line, "kept ") || !strings.Contains(line, "verbatim") {
		t.Errorf("report = %q, want the kept share", line)
	}
	if !strings.Contains(line, "summarized 2 turns") {
		t.Errorf("report = %q, want the summarized turns", line)
	}
	if !strings.Contains(line, "freed") {
		t.Errorf("report = %q, want the freed tokens", line)
	}
}

func TestSettleCompactionInstallsASummary(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	outcome := compactOutcome{before: int64(125_000), evictCut: len(m.conversation) - keepCount(len(m.conversation)), summary: "Decisions:\n- done."}

	m.settleCompaction(outcome)

	if m.compacting {
		t.Errorf("compacting still true after the outcome is installed")
	}
	if len(m.conversation) != len(bigConversation())-2 {
		t.Errorf("conversation = %d messages, want the evicted middle replaced (folded into the kept user turn)", len(m.conversation))
	}
	want := compactApply(bigConversation(), 2, "Decisions:\n- done.", int64(125_000)).after
	if m.size != want {
		t.Errorf("size = %d, want the rebuild's %d", m.size, want)
	}
	if m.size >= outcome.before {
		t.Errorf("size = %d, want a summarized pass to shrink below %d", m.size, outcome.before)
	}
	said := spoken(m)
	if len(said) != 1 {
		t.Fatalf("spoken = %v, want the compaction report alone", said)
	}
	line := strings.Join(strings.Split(said[0], "\n"), " ")
	if !strings.Contains(line, "✂ Compaction summary") || !strings.Contains(line, "summarized") || !strings.Contains(line, "freed") {
		t.Errorf("report = %q, want the compaction card naming the summary", said[0])
	}
}

func TestCompactPromptFeedsExactlyTheEvictedChunk(t *testing.T) {
	conv := bigConversation()
	evictCut := len(conv) - keepCount(len(conv))

	prompt := compactPrompt(conv, evictCut)

	if len(prompt) != evictCut+1 {
		t.Fatalf("summarizer prompt = %d messages, want the %d evicted + the ask", len(prompt), evictCut)
	}
	for i := range evictCut {
		if prompt[i].Role != conv[i].Role || len(prompt[i].Parts) != len(conv[i].Parts) {
			t.Errorf("prompt message %d does not match the evicted chunk message %d", i, i)
		}
	}
	ask, ok := prompt[evictCut].Parts[0].(nacelle.Text)
	if !ok || !strings.Contains(ask.Text, "older turns") {
		t.Errorf("last prompt message = %q, want the compact ask", ask.Text)
	}
	if len(prompt) == len(conv) {
		t.Errorf("prompt fed the whole conversation instead of the evicted chunk")
	}
}

func TestCompactPromptScopesTheSummaryToItsChunk(t *testing.T) {
	if !strings.Contains(compactSystem, "only the turns shown to you") {
		t.Errorf("compactSystem = %q, want an explicit chunk-scoping rule", compactSystem)
	}
	if !strings.Contains(compactSystem, "not part of this request") && !strings.Contains(compactAsk, "not part of this request") {
		t.Errorf("prompts = %q / %q, want an explicit 'not part of this request' boundary", compactSystem, compactAsk)
	}
	for _, section := range []string{"Decisions", "Constraints", "Plan", "State", "Artifacts", "Ruled out", "Open questions"} {
		if !strings.Contains(compactSystem, section) {
			t.Errorf("compactSystem missing the %q section of the schema", section)
		}
	}
	if !strings.Contains(compactSystem, "Never invent facts") {
		t.Errorf("compactSystem = %q, want a no-invention rule", compactSystem)
	}
}
