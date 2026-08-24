package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

func TestTruncateLeavesWhatAlreadyFits(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate = %q, want it unchanged", got)
	}
}

// The bug this was written for. The byte-counting version spent its whole
// budget and then appended the ellipsis on top, so it answered a request for
// ten cells with eleven — one more than the status line has room for, which
// wraps the one row layout() reserves exactly one of.
func TestTruncateNeverReturnsMoreCellsThanItWasAskedFor(t *testing.T) {
	for _, s := range []string{
		"this is much too long",
		"0123456789012345678901234",
		"⠋ running run_command · 12345 tokens · $0.0123",
		"queued · café déjà vu, encore une fois",
		"宽字符宽字符宽字符宽字符宽字符",
	} {
		for _, max := range []int{1, 2, 7, 10, 24} {
			if got := lipgloss.Width(truncate(s, max)); got > max {
				t.Errorf("truncate(%q, %d) is %d cells wide, want at most %d", s, max, got, max)
			}
		}
	}
}

// Cutting on a byte boundary turned "aé" into "a\xc3…". Queued lines are
// whatever was typed, so an accent in a narrow window was all it took.
func TestTruncateNeverSplitsARune(t *testing.T) {
	for _, s := range []string{"aé", "queued · café", "⠋ waiting", "宽字符"} {
		for max := 1; max <= len(s); max++ {
			if got := truncate(s, max); !utf8.ValidString(got) {
				t.Errorf("truncate(%q, %d) = %q, which is not valid UTF-8", s, max, got)
			}
		}
	}
}

// The rune-at-a-time version priced every character of an escape sequence as a
// cell, so a coloured status line was cut a dozen cells early and cut wherever
// the budget ran out — mid-sequence, dropping the reset, which leaks the colour
// into everything printed after it. A styled string is the normal input here.
func TestTruncateChargesNothingForStylingAndAlwaysClosesIt(t *testing.T) {
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render("⠋ waiting for a response")

	if got := truncate(styled, 24); got != styled {
		t.Errorf("truncate = %q, want the line untouched — it is 24 cells of text", got)
	}

	cut := truncate(styled, 12)
	if width := lipgloss.Width(cut); width != 12 {
		t.Errorf("truncate = %q, %d cells wide, want 12", cut, width)
	}
	if !strings.Contains(cut, "waiting") {
		t.Errorf("truncate = %q, want the escape sequence charged nothing and the words kept", cut)
	}
	if opens := strings.Count(cut, "\x1b["); opens != 2 {
		t.Errorf("truncate = %q, want the colour opened and closed exactly once", cut)
	}
}

// Something has to come back for a budget nothing fits in, and an ellipsis
// alone is the honest answer — it says text was cut without claiming any of
// it survived.
func TestTruncateHandlesABudgetOfAlmostNothing(t *testing.T) {
	if got := truncate("anything", 1); got != "…" {
		t.Errorf("truncate = %q, want just the ellipsis", got)
	}
	if got := truncate("anything", 0); got != "" {
		t.Errorf("truncate = %q, want nothing at all", got)
	}
}
