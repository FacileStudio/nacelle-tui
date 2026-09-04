package layout

import (
	"fmt"
	"strings"
	"testing"
)

func TestBudget(t *testing.T) {
	if got := Budget(24, 10); got != 14 {
		t.Errorf("Budget = %d, want 14", got)
	}
	if got := Budget(24, 30); got != 1 {
		t.Errorf("Budget = %d, want floor of 1", got)
	}
}

func TestScrolls(t *testing.T) {
	if got := Scrolls("short", 80); got != 1 {
		t.Errorf("Scrolls = %d, want 1", got)
	}
	longLine := strings.Repeat("x", 165)
	if got := Scrolls(longLine, 80); got != 3 {
		t.Errorf("Scrolls = %d, want 3", got)
	}
}

func TestFits(t *testing.T) {
	lines := []string{"line1", "line2", "line3"}
	if got := Fits(lines, 2, 80); got != 2 {
		t.Errorf("Fits = %d, want 2", got)
	}
	oversized := []string{strings.Repeat("x", 200)}
	if got := Fits(oversized, 1, 80); got != 1 {
		t.Errorf("Fits oversized = %d, want 1", got)
	}
}

func TestBatches(t *testing.T) {
	lines := make([]string, 14)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i)
	}
	batches := Batches(strings.Join(lines, "\n"), 4, 80)
	if len(batches) != 4 {
		t.Fatalf("Batches len = %d, want 4", len(batches))
	}
}

func TestLiveRows(t *testing.T) {
	if got := LiveRows(40, 10); got != 10 {
		t.Errorf("LiveRows = %d, want 10", got)
	}
	if got := LiveRows(10, 20); got != 1 {
		t.Errorf("LiveRows floor = %d, want 1", got)
	}
}

func TestPromptCap(t *testing.T) {
	if got := PromptCap(30); got != 10 {
		t.Errorf("PromptCap = %d, want 10", got)
	}
	if got := PromptCap(2); got != 1 {
		t.Errorf("PromptCap floor = %d, want 1", got)
	}
}
