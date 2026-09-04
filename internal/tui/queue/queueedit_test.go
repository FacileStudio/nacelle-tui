package queue

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestQueueReplace(t *testing.T) {
	q := New()
	q.Add("typo line")
	q.Add("second line")

	q.Replace(0, "fixed line")
	if q.At(0) != "fixed line" {
		t.Errorf("At(0) = %q, want fixed line", q.At(0))
	}
	if q.At(1) != "second line" {
		t.Errorf("At(1) = %q, want second line", q.At(1))
	}
}

func TestQueueWaitingHidesEditedLine(t *testing.T) {
	q := New()
	q.Add("first")
	q.Add("second")

	waiting := q.Waiting(1)
	if len(waiting) != 1 || waiting[0] != "first" {
		t.Errorf("Waiting(1) = %v, want [first]", waiting)
	}
}

func TestQueueHeightExcludesEditedLine(t *testing.T) {
	q := New()
	q.Add("first")
	q.Add("second")

	if h := q.Height(1); h != 1 {
		t.Errorf("Height(editing=1) = %d, want 1", h)
	}
}

func TestQueueViewExcludesEditedLine(t *testing.T) {
	q := New()
	q.Add("first")
	q.Add("second")

	style := lipgloss.NewStyle()
	lines := q.View(1, 80, style)
	if len(lines) != 1 {
		t.Fatalf("View drew %d lines, want 1", len(lines))
	}
	if strings.Contains(lines[0], "second") {
		t.Errorf("View drew edited line: %q", lines[0])
	}
	if !strings.Contains(lines[0], "first") {
		t.Errorf("View missing unedited line: %q", lines[0])
	}
}
