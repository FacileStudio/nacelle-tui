package tui

import (
	"fmt"
	"strings"

	"github.com/FacileStudio/nacelle"
)

const (
	// compactMinResult is the smallest tool result worth masking. Under it,
	// the placeholder costs nearly what the result did, and a result that
	// small is usually load-bearing: an id, a path, a diff header.
	compactMinResult = 1024

	// compactSlack is how far under compactAt one pass aims to land, so a
	// session that grows between passes does not trim on every turn.
	compactSlack = 20_000

	// droppedNotice opens the text a masked result is replaced with. The mask
	// skips any result already opening with it, so a second pass never pays a
	// placeholder twice or counts it as savings.
	droppedNotice = "[dropped "

	// droppedThinkingNotice opens the text a masked thinking block is replaced
	// with. It plays the same role as droppedNotice, and uses the same "dropped"
	// prefix so the reader sees it as the same mechanism.
	droppedThinkingNotice = "[dropped thinking: "
)

// estTokens is the bytes-to-tokens estimate the whole file uses: four bytes
// per token is the rough English rate, and it is only ever compared against
// itself, so its error bars are directionally consistent.
func estTokens(bytes int) int64 {
	return int64(bytes) / 4
}

// convBytes is the total byte weight of the first n messages, a stand-in for
// "what is about to be discarded" when estimating how much a pass frees.
func convBytes(conv []nacelle.Message, n int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += msgBytes(conv[i])
	}
	return total
}

// msgBytes is the byte weight of one message: the sum across its parts, which
// hold the tool output, the thinking and the spoken text, and nothing else
// that carries size worth counting.
func msgBytes(msg nacelle.Message) int {
	total := 0
	for _, part := range msg.Parts {
		total += partBytes(part)
	}
	return total
}

// partBytes is the byte weight of a single part: tool results and reasoning
// dominate a long session, and text is the message itself. Anything else
// weights nothing.
func partBytes(part nacelle.Part) int {
	if text, ok := part.(nacelle.Text); ok {
		return len(text.Text)
	}
	if result, ok := part.(nacelle.ToolResult); ok {
		return len(result.Result)
	}
	if reasoning, ok := part.(nacelle.Reasoning); ok {
		return len(reasoning.Text)
	}
	return 0
}

// maskEvicted is the cheap fallback stage: replace oversized tool results and
// thinking blocks in the evicted middle with placeholders, keeping the
// call/result pairing intact. It is synchronous, needs no backend, and runs on
// the UI thread, so it is the whole of compaction when no model is available
// to summarize. It mutates the conversation in place, which is safe here
// because it runs on the same thread that owns the conversation.
func (m *Model) maskEvicted(evictCut int) (int, int, int64) {
	budget := (m.size - m.compactAt + compactSlack) * 4
	if budget < 0 {
		budget = 0
	}
	results, thinking := 0, 0
	var saved int64
	for i := 0; i < evictCut && i < len(m.conversation); i++ {
		if budget > 0 {
			s, cut := m.trimResults(&m.conversation[i], budget)
			budget -= s
			saved += s
			results += cut
		}
		_, cut := m.trimThinking(&m.conversation[i], 0)
		thinking += cut
	}
	if saved > 0 {
		tokensSaved := saved / 4
		if tokensSaved > m.size {
			m.size = 0
		} else {
			m.size -= tokensSaved
		}
	}
	return results, thinking, saved
}

// trimResults replaces the droppable tool results of one message, up to
// budget bytes of original text, and reports how much it saved and how many
// results it replaced. Only user messages carry tool results, and the guard at
// the top makes this a quick no-op for assistant messages that trimThinking
// handles instead.
func (m *Model) trimResults(message *nacelle.Message, budget int64) (int64, int) {
	if message.Role != nacelle.RoleUser {
		return 0, 0
	}
	saved := int64(0)
	cut := 0
	for j, part := range message.Parts {
		if saved >= budget {
			break
		}
		result, ok := part.(nacelle.ToolResult)
		if !ok || len(result.Result) < compactMinResult || strings.HasPrefix(result.Result, droppedNotice) {
			continue
		}
		saved += int64(len(result.Result))
		message.Parts[j] = nacelle.ToolResult{
			ID:     result.ID,
			Name:   result.Name,
			Failed: result.Failed,
			Result: fmt.Sprintf("%s%d bytes%s", droppedNotice, len(result.Result), ". Re-run the tool if the detail matters."),
		}
		m.trimmed++
		cut++
	}
	return saved, cut
}

// trimThinking replaces long-form reasoning in old assistant messages with a
// placeholder, up to budget bytes of original text, and reports how much it
// saved and how many blocks it replaced. Reasoning that was dropped in an
// earlier pass is left alone.
//
// Thinking blocks are never sent back to the provider — nacelle drops them when
// building the request — so replacing them is invisible on the wire. What the
// model actually said (the Text part) stays untouched.
func (m *Model) trimThinking(message *nacelle.Message, budget int64) (int64, int) {
	if message.Role != nacelle.RoleAssistant {
		return 0, 0
	}
	saved := int64(0)
	cut := 0
	for j, part := range message.Parts {
		if budget > 0 && saved >= budget {
			break
		}
		reasoning, ok := part.(nacelle.Reasoning)
		if !ok || reasoning.Text == "" || strings.HasPrefix(reasoning.Text, droppedThinkingNotice) {
			continue
		}
		saved += int64(len(reasoning.Text))
		message.Parts[j] = nacelle.Reasoning{
			Text: fmt.Sprintf("%s%d bytes%s", droppedThinkingNotice, len(reasoning.Text), ". See the assistant text above for the conclusion."),
		}
		m.trimmed++
		cut++
	}
	return saved, cut
}

// applyMaskFallback runs the mask in place on the UI thread and reports it,
// for the two cases where the summary did not happen: the summarizer errored,
// or produced nothing. The mask still frees the bulky tool output, so a failed
// summary costs nothing but the attempt.
func (m *Model) applyMaskFallback(outcome compactOutcome) {
	evictCut := len(m.conversation) - keepCount(len(m.conversation))
	results, thinking, _ := m.maskEvicted(evictCut)
	done := compacted{
		evictCut: evictCut,
		kept:     len(m.conversation) - evictCut,
		results:  results,
		thinking: thinking,
	}
	report := compactOutcome{
		before:       outcome.before,
		after:        m.size,
		done:         done,
		conversation: m.conversation,
	}
	m.say(fromCompact, compactReport(report))
}
