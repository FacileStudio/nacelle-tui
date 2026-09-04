// Package history manages prompt navigation history and requeueing.
package history

// History tracks sent prompts and navigation through past and queued inputs.
type History struct {
	Past    []string
	Index   int
	Draft   string
	FromEnd int
}

// New creates an empty history tracker.
func New() *History {
	return &History{}
}

// Walkable returns all entries available for history navigation: sent questions, then queued lines.
func (h *History) Walkable(queued []string) []string {
	if len(queued) == 0 {
		return h.Past
	}
	entries := make([]string, 0, len(h.Past)+len(queued))
	return append(append(entries, h.Past...), queued...)
}

// Recall moves one entry back through the walk.
func (h *History) Recall(currentPrompt string, queued []string) (string, bool) {
	entries := h.Walkable(queued)
	if len(entries) == 0 {
		return "", false
	}
	h.Index = min(h.Index, len(entries))
	if h.Index == len(entries) {
		h.Draft = currentPrompt
	}
	if h.Index == 0 {
		return "", false
	}
	h.Index--
	return h.land(entries, queued), true
}

// Advance moves one entry forward, restoring the draft past the newest entry.
func (h *History) Advance(queued []string) (string, bool) {
	entries := h.Walkable(queued)
	if h.Index >= len(entries) {
		return "", false
	}
	h.Index++
	if h.Index == len(entries) {
		h.FromEnd = 0
		return h.Draft, true
	}
	return h.land(entries, queued), true
}

func (h *History) land(entries []string, queued []string) string {
	h.FromEnd = 0
	if q := len(entries) - h.Index; q <= len(queued) {
		h.FromEnd = q
	}
	return entries[h.Index]
}

// Remember records a sent question, deduplicating past entries.
func (h *History) Remember(question string, queued []string) {
	for i, past := range h.Past {
		if past == question {
			h.Past = append(h.Past[:i], h.Past[i+1:]...)
			break
		}
	}
	h.Past = append(h.Past, question)
	h.Index = len(h.Walkable(queued))
	h.Draft = ""
	h.FromEnd = 0
}
