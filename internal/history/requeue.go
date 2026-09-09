package history

import "strings"

// Requeue puts an edited line back where the walk found it.
//
// It only claims the line while the submitted text is still a draft of it —
// one still carries the other's text. A FromEnd left over from browsing history
// while a fresh, unrelated message is being typed is stale: hijacking that
// message as a phantom edit would put it silently back in the queue where the
// user watching the agent settle would read it as swallowed. Related is
// deliberately loose (substring) so a real rewrite mid-edit still lands in
// place; an unrelated message just queues or sends on its own.
func (h *History) Requeue(queued []string, text string) bool {
	if h.FromEnd <= 0 || h.FromEnd > len(queued) {
		return false
	}
	line := queued[len(queued)-h.FromEnd]
	if !related(text, line) {
		return false
	}
	queued[len(queued)-h.FromEnd] = text
	return true
}

// related reports whether one of two prompt lines reads as a draft of the
// other: equal, or one still carrying the other's text. A completely different
// question shares nothing and queues as new rather than quietly replacing the
// line under edit.
func related(text, line string) bool {
	if text == "" || line == "" {
		return false
	}
	return strings.Contains(text, line) || strings.Contains(line, text)
}

// Editing returns the index in queued being edited, or -1 if none.
func (h *History) Editing(queuedLen int) int {
	if h.FromEnd == 0 || h.FromEnd > queuedLen {
		return -1
	}
	return queuedLen - h.FromEnd
}

// Reanchor updates the editing position after a queued line is delivered.
func (h *History) Reanchor(editing, sent, queuedLen int) {
	if editing < 0 {
		return
	}
	if sent < editing {
		editing--
	}
	h.FromEnd = queuedLen - editing
}
