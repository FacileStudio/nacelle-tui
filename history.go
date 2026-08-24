package main

// promptHistory is what the arrow keys walk: the questions already sent, and
// the walk's own position in them.
//
// Up recalls the previous question into the input and each further Up walks
// further back; Down walks forward again, and one step past the newest entry
// restores whatever was being written when the walk started. The draft is
// kept for exactly that round trip and nowhere else — it is not an entry
// until it is sent, because half a thought is not history.
//
// The walk reaches further than past, though: see walkable. Anything still
// queued behind a run in flight is walked before the sent questions are, and
// fromEnd is how the entry the walk landed on is named when it is one of
// those.
type promptHistory struct {
	past    []string
	index   int
	draft   string
	fromEnd int
}

// walkable is everything Up steps back through: the questions already sent,
// oldest first, then the lines still queued behind the run in flight.
//
// One flat list rather than a cursor per source. The walk is the same walk
// either way — step back from the prompt, step forward to the draft — and the
// only thing a queue changes is what the last few entries are. Two cursors
// would be two places to hold a position, two ways to be off by one, and a
// seam in the middle of a keypress that has to behave as if there were none.
//
// Queued lines go last because the walk runs backwards from the prompt. The
// line typed most recently is the first one Up reaches, and a line still
// waiting to be sent is both more recent than anything already sent and the
// only kind that can still be changed — so it is the one somebody pressing Up
// is reaching for.
func (m *model) walkable() []string {
	if len(m.run.queued) == 0 {
		return m.hist.past
	}
	entries := make([]string, 0, len(m.hist.past)+len(m.run.queued))
	return append(append(entries, m.hist.past...), m.run.queued...)
}

// recall moves one entry back through the walk, reporting whether it claimed
// the press. It declines when there is nothing to recall and when the walk is
// already on the oldest entry there is.
//
// That second guard is not defensive dressing. Without it the index ran on
// past zero and the next press indexed the list at -1, which panicked the
// whole client — one question sent and two Ups was the entire reproduction.
// Declining hands the press to the textarea, where "up" on the first row is
// already a no-op, so the key does nothing rather than taking the session
// down with it.
//
// The index is clamped on the way in because the list it walks can shrink
// underneath it: settle delivers queued lines while somebody is reading, so a
// position that was inside the list when it was taken can be past the end of
// it by the next press.
func (m *model) recall() bool {
	entries := m.walkable()
	if len(entries) == 0 {
		return false
	}
	m.hist.index = min(m.hist.index, len(entries))
	if m.hist.index == len(entries) {
		m.hist.draft = m.prompt.Value()
	}
	if m.hist.index == 0 {
		return false
	}
	m.hist.index--
	m.land(entries)
	return true
}

// advance moves one entry forward, restoring the draft past the newest entry,
// and reports whether it claimed the press.
func (m *model) advance() bool {
	entries := m.walkable()
	if m.hist.index >= len(entries) {
		return false
	}
	m.hist.index++
	if m.hist.index == len(entries) {
		m.hist.fromEnd = 0
		m.setEntry(m.hist.draft)
		return true
	}
	m.land(entries)
	return true
}

// land puts the entry the walk stopped on into the input, and records whether
// that entry is a queued line rather than a sent one.
//
// Queued lines are named by their distance from the end of the queue, never by
// their index in it, and the difference is the whole reason editing a queued
// line is safe. deliver drains the queue from the front, so a run settling
// while somebody is part way through an edit shifts every index down — an
// offset from the back names the same line however many go out in front of it.
// Reaching the end of the queue is then the one thing that can lose it, and
// that is exactly the case requeue has to refuse.
func (m *model) land(entries []string) {
	m.hist.fromEnd = 0
	if queued := len(entries) - m.hist.index; queued <= len(m.run.queued) {
		m.hist.fromEnd = queued
	}
	m.setEntry(entries[m.hist.index])
}

// requeue puts an edited line back where the walk found it, and reports
// whether it belonged to the queue at all.
//
// The offset is re-checked rather than trusted. A run can settle at any point
// between the arrow key and the enter, and deliver sends from the front of the
// queue, so the line being edited may already have gone out by the time the
// edit is finished. Then there is nothing to put back: the edit becomes a new
// message, which is the only honest answer available, because the version
// already sent cannot be unsent.
func (m *model) requeue(text string) bool {
	if m.hist.fromEnd == 0 || m.hist.fromEnd > len(m.run.queued) {
		return false
	}
	m.run.queued[len(m.run.queued)-m.hist.fromEnd] = text
	return true
}

// remember files one sent question at the end of the history and ends any
// walk through it. Duplicates are moved rather than re-appended, so pressing
// Up from the prompt offers the question actually asked last, however long
// this session has been running.
//
// The position it leaves behind is the end of the whole walk, not the end of
// past, which is why it has to run after the queue has been added to rather
// than before — see ask. Left at the end of past with lines still queued, the
// next Up would step into the middle of the queue instead of onto the line
// most recently typed.
func (m *model) remember(question string) {
	for i, past := range m.hist.past {
		if past == question {
			m.hist.past = append(m.hist.past[:i], m.hist.past[i+1:]...)
			break
		}
	}
	m.hist.past = append(m.hist.past, question)
	m.hist.index = len(m.walkable())
	m.hist.draft = ""
	m.hist.fromEnd = 0
}

// setEntry puts one recalled line in the input with the caret at the end,
// which is where somebody pressing Up expects their next keystroke to land.
func (m *model) setEntry(value string) {
	m.prompt.Reset()
	m.prompt.SetValue(value)
	m.prompt.MoveToEnd()
}
