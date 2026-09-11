// Package tui — the light lever and the thrash guard of context compaction.
package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// thrashLimit is how many consecutive compaction passes must fail to land the
// conversation under the trigger threshold before the automatic triggers back
// off. A single failed pass is not a reason to stop trying — it can be a
// transient summarizer rejection — so the guard only stands down once the
// pattern is clear that a pass cannot help. Three is Claude Code's own number
// for the same guard.
const thrashLimit = 3

// compactBeforeSend is the pre-flight entry from send. Called when CountTokens
// shows the next turn over the trigger threshold, it runs the cheap, backend-free
// mask stage first and only escalates to the summarizer pass when masking alone
// cannot land the conversation under the threshold. Masking is the lightest-touch
// form of compaction (the field's documented order: clear older tool outputs
// first, then summarize if still needed), and it needs no model call, so a pass
// that masking clears costs nothing but a byte sweep. When masking cannot clear it
// — usually because the bloat lives in the kept tail, which a pass never evicts —
// it hands off to the full summarize pass.
func (m *Model) compactBeforeSend(ctx context.Context) tea.Cmd {
	evictCut := alignedEvictCut(m.conversation, len(m.conversation)-keepCount(len(m.conversation)))
	if evictCut <= 0 {
		return nil
	}
	if m.maskLight(evictCut) {
		return nil
	}
	return m.beginCompaction(ctx)
}

// maskLight runs one mask pass and reports it, returning true when it landed the
// conversation under the trigger threshold so no further pass is needed. The mask
// trims oversized tool results and thinking blocks in the evicted middle and
// tightens m.size to the estimated result, so "did it get under?" is the same
// comparison the send path uses to decide whether to compact at all. A pass that
// masked nothing and still sits over the threshold returns false, leaving the
// caller to escalate.
func (m *Model) maskLight(evictCut int) bool {
	before := m.size
	results, thinking, _ := m.maskEvicted(evictCut)
	if m.size > m.compactAt+compactSlack {
		return false
	}
	m.thrashCount = 0
	if results > 0 || thinking > 0 {
		m.say(fromCompact, compactReport(compactOutcome{
			before: before,
			after:  m.size,
			done:   compacted{evictCut: evictCut, kept: len(m.conversation) - evictCut, results: results, thinking: thinking},
		}))
	}
	return true
}

// thrashed is whether the automatic triggers should back off: enough consecutive
// passes have failed to land the conversation under the threshold that repeating
// a pass is more likely to waste a call than to help. A manual /compact still
// forces a fresh attempt, and a pass that lands under, or a /clear, resets it.
func (m *Model) thrashed() bool {
	return m.thrashCount >= thrashLimit
}

// checkThrash closes a pass that left the conversation still over the trigger
// threshold. Rather than backing off on the first miss, it counts: each pass that
// fails to land under increments, and each one that does resets. Only when the
// count reaches thrashLimit does it warn and stand the automatic triggers down —
// a single enormous result, usually in the kept tail eviction never touches, is
// the class a pass cannot clear, and repeating it cannot help. What a reader can
// act on is named in the notice: /clear, or reading in chunks, then a manual
// /compact.
func (m *Model) checkThrash() {
	if m.compactAt > 0 && m.size > m.compactAt+compactSlack {
		m.thrashCount++
		if m.thrashCount == thrashLimit {
			m.say(fromClient, "compaction keeps leaving the context over the threshold — one very large result is likeliest; /clear, read in chunks, then /compact")
		}
		return
	}
	m.thrashCount = 0
}
