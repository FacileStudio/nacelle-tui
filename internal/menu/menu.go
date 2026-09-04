// Package menu implements the popup autocomplete menu for slash commands.
package menu

// MaxMenuItems caps how many matches the dropdown draws at once.
const MaxMenuItems = 8

// Item is one candidate row in the autocomplete dropdown.
type Item struct {
	Value       string
	Description string
}

// Menu tracks the candidates and navigation state of the autocomplete popup.
type Menu struct {
	Items     []Item
	Filtered  []Item
	Selected  int
	Scroll    int
	Dismissed bool
}

// New creates a Menu with the provided candidate items.
func New(items []Item) *Menu {
	return &Menu{Items: items}
}

// Open reports whether the dropdown is currently visible.
func (m Menu) Open() bool {
	return !m.Dismissed && len(m.Filtered) > 0
}

// Height returns how many rows the dropdown draws.
func (m Menu) Height() int {
	if !m.Open() {
		return 0
	}
	return min(len(m.Filtered), MaxMenuItems)
}
