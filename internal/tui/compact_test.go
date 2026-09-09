package tui

import (
	"errors"
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

func TestApplyReplacesTheEvictedMiddleWithASummary(t *testing.T) {
	conv := bigConversation()
	evictCut := len(conv) - keepCount(len(conv))

	outcome := compactApply(conv, evictCut, "Decisions:\n- went with the read.", int64(125_000))

	// The oldest kept message is a user tool-result turn, so the summary is
	// folded into it as a leading block; the message count is the kept tail,
	// not len(conv)-evictCut+1.
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
	// The kept tail survives verbatim and in order, including the user turn the
	// summary merged into (which keeps its own tool-result parts).
	for i := 0; i < len(outcome.conversation); i++ {
		if outcome.conversation[i].Role != conv[evictCut+i].Role {
			t.Errorf("message %d of the tail does not match the kept message %d", i, evictCut+i)
		}
	}
	lastResult, lastOk := first.Parts[len(first.Parts)-1].(nacelle.ToolResult)
	if !lastOk || lastResult.ID != "old-2" {
		t.Errorf("the merged user turn lost its original tool result: %v", first.Parts)
	}
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

func TestCompactedConversationKeepsRolesAlternating(t *testing.T) {
	// A tool-result reply is a user message, and it is usually the oldest kept
	// message, so folding the summary in must never leave two user turns in a
	// row — the backends reject that. The other parity (first kept is an
	// assistant turn) must insert the summary as its own preceding user turn.
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
	// Regression: the mask used to run before the summarizer, so the model
	// saw "[dropped N bytes]" placeholders instead of the tool output it was
	// supposed to preserve. The summarizer must read the untouched chunk.
	conv := bigConversation()
	evictCut := len(conv) - keepCount(len(conv))

	prompt := compactPrompt(conv, evictCut)

	for i := 0; i < evictCut; i++ {
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

func TestMaskFallbackKeepsTheConversationStanding(t *testing.T) {
	// Empty summary (offline, or the model returned nothing): the pass falls
	// back to masking in place, which frees headroom without growing anything.
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000
	m.compactAt = compactAt

	outcome := compactOutcome{before: m.size}
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

func TestCompactReportNamesTheWholePass(t *testing.T) {
	outcome := compactApply(bigConversation(), 2, "Decisions:\n- done.", int64(125_000))

	line := compactReport(outcome)

	if !strings.Contains(line, "✂") {
		t.Errorf("report = %q, want the compaction icon", line)
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
	outcome := compactOutcome{before: int64(125_000), summary: "Decisions:\n- done."}

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
	if !strings.Contains(line, "✂ compacted context") || !strings.Contains(line, "summarized") || !strings.Contains(line, "freed") {
		t.Errorf("report = %q, want the compaction line naming the summary", said[0])
	}
}

func TestSettleCompactionFallsBackToTheMaskOnFailure(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000
	outcome := compactOutcome{before: m.size, err: errors.New("summarizer hiccuped")}

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

func TestCompactPromptFeedsExactlyTheEvictedChunk(t *testing.T) {
	conv := bigConversation()
	evictCut := len(conv) - keepCount(len(conv)) // 2

	prompt := compactPrompt(conv, evictCut)

	if len(prompt) != evictCut+1 {
		t.Fatalf("summarizer prompt = %d messages, want the %d evicted + the ask", len(prompt), evictCut)
	}
	// The evicted chunk arrives verbatim and first, in order.
	for i := 0; i < evictCut; i++ {
		if prompt[i].Role != conv[i].Role || len(prompt[i].Parts) != len(conv[i].Parts) {
			t.Errorf("prompt message %d does not match the evicted chunk message %d", i, i)
		}
	}
	// The ask is last; the kept tail never reaches the summarizer.
	ask, ok := prompt[evictCut].Parts[0].(nacelle.Text)
	if !ok || !strings.Contains(ask.Text, "older turns") {
		t.Errorf("last prompt message = %q, want the compact ask", ask.Text)
	}
	if len(prompt) == len(conv) {
		t.Errorf("prompt fed the whole conversation instead of the evicted chunk")
	}
}

func TestCompactPromptScopesTheSummaryToItsChunk(t *testing.T) {
	// The system and the ask must both name the chunk boundary and forbid
	// reaching past it, or the model will "recap the whole conversation".
	if !strings.Contains(compactSystem, "only the turns shown to you") {
		t.Errorf("compactSystem = %q, want an explicit chunk-scoping rule", compactSystem)
	}
	if !strings.Contains(compactSystem, "not part of this request") && !strings.Contains(compactAsk, "not part of this request") {
		t.Errorf("prompts = %q / %q, want an explicit 'not part of this request' boundary", compactSystem, compactAsk)
	}
	// Not a dumb one-liner: the schema must carry the load-bearing sections.
	for _, section := range []string{"Decisions", "Constraints", "Plan", "State", "Artifacts", "Ruled out", "Open questions"} {
		if !strings.Contains(compactSystem, section) {
			t.Errorf("compactSystem missing the %q section of the schema", section)
		}
	}
	// No invention rule: the inventory of facts must be grounded in the source.
	if !strings.Contains(compactSystem, "Never invent facts") {
		t.Errorf("compactSystem = %q, want a no-invention rule", compactSystem)
	}
}

func TestCompactedHistoryCarriesTheMarker(t *testing.T) {
	block := compactedHistory("Decisions:\n- ship it")
	text, ok := block.Parts[0].(nacelle.Text)
	if !ok || !strings.HasPrefix(text.Text, "[compacted context") {
		t.Errorf("compacted history = %q, want the marker prefix", text.Text)
	}
}

func TestSizedCountsEveryBilledInputKind(t *testing.T) {
	m := sized()
	m.sized(nacelle.Usage{InputTokens: 1000, CacheReadTokens: 9000, CacheCreationTokens: 500})
	if m.size != 10_500 {
		t.Errorf("size = %d, want cache reads and creations billed as input", m.size)
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
