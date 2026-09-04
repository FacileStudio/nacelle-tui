package menu

import (
	"sort"
	"strings"
)

// Filter updates the filtered list of items according to query.
func (m *Menu) Filter(query string) {
	m.Filtered = FilterMenu(m.Items, query)
	if TypedOut(m.Filtered, query) {
		m.Filtered = nil
	}
	if m.Selected >= len(m.Filtered) {
		m.Selected = 0
	}
	m.ClampView()
}

// FilterMenu narrows items to what query matches, ranked best first.
func FilterMenu(items []Item, query string) []Item {
	if query == "" {
		return items
	}

	type scored struct {
		item Item
		rank int
	}
	matches := make([]scored, 0, len(items))
	for _, it := range items {
		if rank := MatchRank(it.Value, query); rank >= 0 {
			matches = append(matches, scored{it, rank})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].rank < matches[j].rank })

	out := make([]Item, len(matches))
	for i, s := range matches {
		out[i] = s.item
	}
	return out
}

// MatchRank scores how well candidate matches query: 0 prefix, 1 substring, 2 fuzzy, -1 none.
func MatchRank(candidate, query string) int {
	c, q := strings.ToLower(candidate), strings.ToLower(query)
	switch {
	case strings.HasPrefix(c, q):
		return 0
	case strings.Contains(c, q):
		return 1
	case FuzzyMatch(c, q):
		return 2
	default:
		return -1
	}
}

// FuzzyMatch reports whether every byte of query appears in candidate in order.
func FuzzyMatch(candidate, query string) bool {
	i := 0
	for j := 0; j < len(candidate) && i < len(query); j++ {
		if candidate[j] == query[i] {
			i++
		}
	}
	return i == len(query)
}

// TypedOut reports whether query matches an item value exactly.
func TypedOut(items []Item, query string) bool {
	for _, it := range items {
		if it.Value == query {
			return true
		}
	}
	return false
}
