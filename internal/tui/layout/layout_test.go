package layout

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

func TestTruncateLeavesWhatAlreadyFits(t *testing.T) {
	if got := Truncate("short", 10); got != "short" {
		t.Errorf("Truncate = %q, want it unchanged", got)
	}
}

func TestTruncateNeverReturnsMoreCellsThanItWasAskedFor(t *testing.T) {
	for _, s := range []string{
		"this is much too long",
		"0123456789012345678901234",
		"⠋ running run_command · 12345 tokens · $0.0123",
		"queued · café déjà vu, encore une fois",
		"宽字符宽字符宽字符宽字符宽字符",
	} {
		for _, maxCells := range []int{1, 2, 7, 10, 24} {
			if got := lipgloss.Width(Truncate(s, maxCells)); got > maxCells {
				t.Errorf("Truncate(%q, %d) is %d cells wide, want at most %d", s, maxCells, got, maxCells)
			}
		}
	}
}

func TestTruncateNeverSplitsARune(t *testing.T) {
	for _, s := range []string{"aé", "queued · café", "⠋ waiting", "宽字符"} {
		for maxLen := 1; maxLen <= len(s); maxLen++ {
			if got := Truncate(s, maxLen); !utf8.ValidString(got) {
				t.Errorf("Truncate(%q, %d) = %q, not valid UTF-8", s, maxLen, got)
			}
		}
	}
}

func TestTruncateChargesNothingForStylingAndAlwaysClosesIt(t *testing.T) {
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render("⠋ waiting for a response")

	if got := Truncate(styled, 24); got != styled {
		t.Errorf("Truncate = %q, want untouched", got)
	}

	cut := Truncate(styled, 12)
	if width := lipgloss.Width(cut); width != 12 {
		t.Errorf("Truncate = %q, %d cells wide, want 12", cut, width)
	}
	if !strings.Contains(cut, "waiting") {
		t.Errorf("Truncate = %q, want words kept", cut)
	}
	if opens := strings.Count(cut, "\x1b["); opens != 2 {
		t.Errorf("Truncate = %q, want colour opened and closed once", cut)
	}
}

func TestTruncateHandlesABudgetOfAlmostNothing(t *testing.T) {
	if got := Truncate("anything", 1); got != "…" {
		t.Errorf("Truncate = %q, want just ellipsis", got)
	}
	if got := Truncate("anything", 0); got != "" {
		t.Errorf("Truncate = %q, want empty string", got)
	}
}

func TestUnstyledStripsEscapes(t *testing.T) {
	styled := lipgloss.NewStyle().Bold(true).Render("plain text")
	if got := Unstyled(styled); got != "plain text" {
		t.Errorf("Unstyled(%q) = %q, want plain text", styled, got)
	}
}

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
