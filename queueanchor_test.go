package main

import "testing"

// What keeps the edit offset pointing at the same line while the queue drains
// around it. Split from the queue tests because filet caps a file at 250 lines.

// The offset naming the line being edited counts from the end of the queue,
// which survives deliver draining the front. Skipping the edited line broke
// that: deliver takes lines from behind it too, and each one shortens the queue
// without moving the edited line closer to the end. The offset then pointed
// past the queue, nothing looked like it was being edited, and the next turn
// of the loop sent the line the reader was still rewriting.
func TestTheEditAnchorSurvivesALaterLineGoingOut(t *testing.T) {
	m := busyWith("being edited", "goes out")
	m.hist.index = len(m.walkable())
	m.key(up)
	m.key(up)

	if m.prompt.Value() != "being edited" || m.editing() != 0 {
		t.Fatalf("prompt = %q editing = %d, want the first queued line", m.prompt.Value(), m.editing())
	}

	editing, at := m.editing(), m.nextToSend()
	if at != 1 {
		t.Fatalf("nextToSend = %d, want the line that is not being edited", at)
	}
	m.run.queued = append(m.run.queued[:at], m.run.queued[at+1:]...)
	m.reanchor(editing, at)

	if got := m.editing(); got != 0 {
		t.Errorf("editing = %d, want 0 — the line in the prompt is still queued", got)
	}
	if next := m.nextToSend(); next != -1 {
		t.Errorf("nextToSend = %d, want nothing sendable: the only line left is being edited", next)
	}
}

// The mirror case: a line in front of the edited one goes out, which shifts it
// down an index while leaving its distance from the end alone.
func TestTheEditAnchorSurvivesAnEarlierLineGoingOut(t *testing.T) {
	m := busyWith("goes out", "being edited")
	m.hist.index = len(m.walkable())
	m.key(up)

	editing, at := m.editing(), m.nextToSend()
	if editing != 1 || at != 0 {
		t.Fatalf("editing = %d, nextToSend = %d, want the last line edited and the first sent", editing, at)
	}
	m.run.queued = append(m.run.queued[:at], m.run.queued[at+1:]...)
	m.reanchor(editing, at)

	if got := m.editing(); got != 0 {
		t.Errorf("editing = %d, want 0 — the edited line shifted down one", got)
	}
	if m.run.queued[0] != "being edited" {
		t.Errorf("queued = %q, want the edited line still there", m.run.queued)
	}
}
