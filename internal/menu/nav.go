package menu

// ClampView adjusts the visible window so the selected item remains on screen.
func (m *Menu) ClampView() {
	if m.Height() == 0 {
		m.Scroll = 0
		return
	}
	if m.Selected < m.Scroll {
		m.Scroll = m.Selected
	}
	if m.Selected >= m.Scroll+m.Height() {
		m.Scroll = m.Selected - m.Height() + 1
	}
}

// Reset clears the filter and dismissal status.
func (m *Menu) Reset() {
	m.Filtered = nil
	m.Dismissed = false
	m.Selected = 0
	m.Scroll = 0
}

// Up moves selection to the previous item.
func (m *Menu) Up() {
	m.Selected = max(m.Selected-1, 0)
	m.ClampView()
}

// Down moves selection to the next item.
func (m *Menu) Down() {
	m.Selected = min(m.Selected+1, len(m.Filtered)-1)
	m.ClampView()
}

// Dismiss closes the menu until re-opened.
func (m *Menu) Dismiss() {
	m.Dismissed = true
}

// SelectedItem returns the currently highlighted item, if any.
func (m *Menu) SelectedItem() (Item, bool) {
	if m.Selected < 0 || m.Selected >= len(m.Filtered) {
		return Item{}, false
	}
	return m.Filtered[m.Selected], true
}
