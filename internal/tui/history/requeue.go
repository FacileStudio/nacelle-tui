package history

// Requeue puts an edited line back where the walk found it.
func (h *History) Requeue(queued []string, text string) bool {
	if h.FromEnd == 0 || h.FromEnd > len(queued) {
		return false
	}
	queued[len(queued)-h.FromEnd] = text
	return true
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
