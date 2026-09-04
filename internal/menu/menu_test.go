package menu

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestFuzzyMatchFindsAnOrderPreservingSubsequence(t *testing.T) {
	if !FuzzyMatch("hunk-review", "hkrev") {
		t.Error(`FuzzyMatch("hunk-review", "hkrev") = false, want true`)
	}
	if strings.Contains("hunk-review", "hkrev") {
		t.Fatal("hkrev is a literal substring")
	}
}

func TestFuzzyMatchRejectsOutOfOrderCharacters(t *testing.T) {
	if FuzzyMatch("review", "vre") {
		t.Error(`FuzzyMatch("review", "vre") = true, want false`)
	}
}

func TestMatchRankPrefersPrefixOverSubstringOverFuzzy(t *testing.T) {
	cases := []struct {
		name      string
		candidate string
		query     string
		want      int
	}{
		{"prefix", "/skill:review", "/skill:rev", 0},
		{"substring", "/skill:facile-review", "review", 1},
		{"fuzzy", "/skill:hunk-review", "hkrev", 2},
		{"no match", "/skill:filet", "xyz", -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatchRank(c.candidate, c.query); got != c.want {
				t.Errorf("MatchRank(%q, %q) = %d, want %d", c.candidate, c.query, got, c.want)
			}
		})
	}
}

func TestFilterMenuRanksBestMatchesFirst(t *testing.T) {
	items := []Item{
		{Value: "/skill:hunk-review"},
		{Value: "/skill:review"},
		{Value: "/skill:facile-review"},
	}

	got := FilterMenu(items, "/skill:rev")
	if len(got) != 3 || got[0].Value != "/skill:review" {
		t.Fatalf("FilterMenu order = %v, want prefix match first", got)
	}
}

func TestFilterMenuWithEmptyQueryReturnsEverything(t *testing.T) {
	items := []Item{{Value: "/clear"}, {Value: "/help"}}
	got := FilterMenu(items, "")
	if len(got) != len(items) {
		t.Errorf("FilterMenu(items, \"\") = %v, want every item", got)
	}
}

func TestCommandMenuOpenRequiresFilteredItemsAndNotDismissed(t *testing.T) {
	m1 := New(nil)
	if m1.Open() {
		t.Error("empty menu should not be open")
	}
	m2 := New(nil)
	m2.Filtered = []Item{{Value: "/clear"}}
	m2.Dismiss()
	if m2.Open() {
		t.Error("dismissed menu should not be open")
	}
	m3 := New(nil)
	m3.Filtered = []Item{{Value: "/clear"}}
	if !m3.Open() {
		t.Error("filtered menu should be open")
	}
}

func TestCommandMenuHeightCapsAtMaxMenuItems(t *testing.T) {
	m := New(nil)
	m.Filtered = make([]Item, MaxMenuItems+5)
	if got := m.Height(); got != MaxMenuItems {
		t.Errorf("Height() = %d, want %d", got, MaxMenuItems)
	}
}

func TestClampViewKeepsTheSelectedRowOnScreen(t *testing.T) {
	m := New(nil)
	m.Filtered = make([]Item, MaxMenuItems+5)
	for i := range m.Filtered {
		m.Filtered[i] = Item{Value: fmt.Sprintf("/cmd:%d", i)}
	}

	m.Selected = len(m.Filtered) - 1
	m.ClampView()

	if m.Scroll > m.Selected-m.Height()+1 {
		t.Errorf("scroll = %d, want window to end on selected row", m.Scroll)
	}
	if m.Scroll+m.Height() <= m.Selected {
		t.Errorf("selected row not in window")
	}
}

func TestMenuRowNeverExceedsWidth(t *testing.T) {
	it := Item{Value: "/skill:antenne", Description: strings.Repeat("x", 200)}
	for _, width := range []int{20, 40, 63, 80} {
		if got := MenuRow(it, width, lipgloss.NewStyle()); len(got) > width {
			t.Errorf("MenuRow(width=%d) = %q, want it to fit", width, got)
		}
	}
}

func TestMenuRowDropsDescriptionWhenNarrow(t *testing.T) {
	it := Item{Value: "/skill:facile-review", Description: "a description"}
	if got := MenuRow(it, 10, lipgloss.NewStyle()); got != it.Value {
		t.Errorf("MenuRow(width=10) = %q, want value only", got)
	}
}
