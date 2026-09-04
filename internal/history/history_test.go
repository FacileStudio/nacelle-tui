package history

import (
	"testing"
)

func TestRecallAndAdvance(t *testing.T) {
	h := New()
	h.Remember("first", nil)
	h.Remember("second", nil)

	val, ok := h.Recall("draft", nil)
	if !ok || val != "second" {
		t.Fatalf("Recall() = %q, %v, want second, true", val, ok)
	}

	val, ok = h.Recall("draft", nil)
	if !ok || val != "first" {
		t.Fatalf("Recall() = %q, %v, want first, true", val, ok)
	}

	_, ok = h.Recall("draft", nil)
	if ok {
		t.Fatal("Recall() at beginning should return false")
	}

	val, ok = h.Advance(nil)
	if !ok || val != "second" {
		t.Fatalf("Advance() = %q, %v, want second, true", val, ok)
	}

	val, ok = h.Advance(nil)
	if !ok || val != "draft" {
		t.Fatalf("Advance() = %q, %v, want draft, true", val, ok)
	}

	_, ok = h.Advance(nil)
	if ok {
		t.Fatal("Advance() at end should return false")
	}
}

func TestRememberDeduplicates(t *testing.T) {
	h := New()
	h.Remember("one", nil)
	h.Remember("two", nil)
	h.Remember("one", nil)

	past := h.Past
	if len(past) != 2 || past[0] != "two" || past[1] != "one" {
		t.Fatalf("past = %v, want [two one]", past)
	}
}

func TestRequeueAndEditing(t *testing.T) {
	h := New()
	h.Remember("sent", nil)
	queued := []string{"q1", "q2"}
	h.Index = len(h.Walkable(queued))

	val, ok := h.Recall("", queued)
	if !ok || val != "q2" {
		t.Fatalf("Recall() = %q, want q2", val)
	}
	if ed := h.Editing(len(queued)); ed != 1 {
		t.Fatalf("Editing() = %d, want 1", ed)
	}

	if !h.Requeue(queued, "edited-q2") {
		t.Fatal("Requeue() failed")
	}
	if queued[1] != "edited-q2" {
		t.Fatalf("queued[1] = %q, want edited-q2", queued[1])
	}
}
