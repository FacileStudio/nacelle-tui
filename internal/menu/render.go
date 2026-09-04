package menu

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const truncationSuffix = "…"

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	return ansi.Truncate(s, max, truncationSuffix)
}

// MenuRow renders a single menu item formatted to width.
func MenuRow(it Item, width int, cmdStyle lipgloss.Style) string {
	value := it.Value
	if strings.HasPrefix(value, "/") {
		value = cmdStyle.Render(value)
	}
	if it.Description == "" {
		return value
	}
	const separator = "  "
	budget := width - lipgloss.Width(value) - len(separator) - len(truncationSuffix)
	if budget < 10 {
		return value
	}
	return value + separator + truncate(it.Description, budget)
}

// View draws the dropdown menu items.
func View(m *Menu, width int, plainStyle, menuStyle, cmdStyle lipgloss.Style) string {
	if !m.Open() {
		return ""
	}
	items := m.Filtered[m.Scroll : m.Scroll+m.Height()]
	rows := make([]string, len(items))
	for i, it := range items {
		style := plainStyle
		marker := "  "
		if m.Scroll+i == m.Selected {
			style = menuStyle
			marker = "→ "
		}
		rows[i] = style.Width(width).Render(marker + MenuRow(it, width-2, cmdStyle))
	}
	return strings.Join(rows, "\n")
}
