package queue

import "testing"

func TestEditingOffset(t *testing.T) {
	q := New()
	q.Add("first")
	q.Add("second")
	q.Add("third")

	if at := q.Editing(1); at != 2 {
		t.Errorf("Editing(fromEnd=1) = %d, want 2 (third)", at)
	}
	if at := q.Editing(3); at != 0 {
		t.Errorf("Editing(fromEnd=3) = %d, want 0 (first)", at)
	}
	if at := q.Editing(0); at != -1 {
		t.Errorf("Editing(fromEnd=0) = %d, want -1", at)
	}
	if at := q.Editing(4); at != -1 {
		t.Errorf("Editing(fromEnd=4) = %d, want -1", at)
	}
}

func TestNextToSend(t *testing.T) {
	q := New()
	q.Add("zero")
	q.Add("one")

	if at := q.NextToSend(-1); at != 0 {
		t.Errorf("NextToSend(not editing) = %d, want 0", at)
	}
	if at := q.NextToSend(0); at != 1 {
		t.Errorf("NextToSend(editing 0) = %d, want 1", at)
	}
	q.PopAt(1)
	if at := q.NextToSend(0); at != -1 {
		t.Errorf("NextToSend(editing only item) = %d, want -1", at)
	}
}

func TestAnchorSurvivesLaterLineGoingOut(t *testing.T) {
	q := New()
	q.Add("being edited")
	q.Add("goes out")

	fromEnd := 2
	editing := q.Editing(fromEnd)
	if editing != 0 {
		t.Fatalf("editing = %d, want 0", editing)
	}

	at := q.NextToSend(editing)
	if at != 1 {
		t.Fatalf("nextToSend = %d, want 1", at)
	}

	q.PopAt(at)
	newFromEnd := q.Len() - editing
	if newEditing := q.Editing(newFromEnd); newEditing != 0 {
		t.Errorf("reanchored editing = %d, want 0", newEditing)
	}
	if next := q.NextToSend(0); next != -1 {
		t.Errorf("nextToSend = %d, want -1", next)
	}
}
