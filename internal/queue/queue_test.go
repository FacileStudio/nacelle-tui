package queue

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestQueueAddAndLen(t *testing.T) {
	q := New()
	if q.Len() != 0 {
		t.Errorf("Len = %d, want 0", q.Len())
	}
	q.Add("first")
	q.Add("second")
	if q.Len() != 2 {
		t.Fatalf("Len = %d, want 2", q.Len())
	}
	if q.At(0) != "first" || q.At(1) != "second" {
		t.Errorf("items = %v, want [first second]", q.Items())
	}
}

func TestQueueDrop(t *testing.T) {
	q := New()
	q.Add("one")
	q.Add("two")
	dropped := q.Drop()
	if dropped != 2 {
		t.Errorf("dropped = %d, want 2", dropped)
	}
	if q.Len() != 0 {
		t.Errorf("Len after drop = %d, want 0", q.Len())
	}
}

func TestQueuePopAtAndAt(t *testing.T) {
	q := New()
	q.Add("zero")
	q.Add("one")
	q.Add("two")
	item := q.PopAt(1)
	if item != "one" {
		t.Errorf("PopAt(1) = %q, want one", item)
	}
	if q.Len() != 2 || q.At(0) != "zero" || q.At(1) != "two" {
		t.Errorf("remaining = %v, want [zero two]", q.Items())
	}
}

func TestQueueWaiting(t *testing.T) {
	q := New()
	q.Add("a")
	q.Add("b")
	q.Add("c")
	all := q.Waiting(-1)
	if len(all) != 3 {
		t.Errorf("Waiting(-1) len = %d, want 3", len(all))
	}
	withoutB := q.Waiting(1)
	if len(withoutB) != 2 || withoutB[0] != "a" || withoutB[1] != "c" {
		t.Errorf("Waiting(1) = %v, want [a c]", withoutB)
	}
}

func TestQueueHeight(t *testing.T) {
	q := New()
	if h := q.Height(-1); h != 0 {
		t.Errorf("Height(0 items) = %d, want 0", h)
	}
	for i := 1; i <= 3; i++ {
		q.Add("item")
		if h := q.Height(-1); h != i {
			t.Errorf("Height(%d items) = %d, want %d", i, h, i)
		}
	}
	q.Add("fourth")
	if h := q.Height(-1); h != MaxRows+1 {
		t.Errorf("Height(4 items) = %d, want %d", h, MaxRows+1)
	}
}

func TestQueueView(t *testing.T) {
	q := New()
	style := lipgloss.NewStyle()
	if len(q.View(-1, 80, style)) != 0 {
		t.Errorf("View on empty queue drew lines")
	}
	q.Add("first question")
	lines := q.View(-1, 80, style)
	if len(lines) != 1 || !strings.Contains(lines[0], "first question") {
		t.Errorf("lines = %v, want [first question]", lines)
	}
}

func TestQueueViewOverflow(t *testing.T) {
	q := New()
	style := lipgloss.NewStyle()
	for range 5 {
		q.Add("item")
	}
	lines := q.View(-1, 80, style)
	if len(lines) != MaxRows+1 {
		t.Fatalf("View drew %d lines, want %d", len(lines), MaxRows+1)
	}
	if !strings.Contains(lines[MaxRows], "and 2 more") {
		t.Errorf("last line = %q, want overflow count", lines[MaxRows])
	}
}
