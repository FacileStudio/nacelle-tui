package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle-tui/internal/tui/menu"
)

func menuItems(skills map[string]skill) []menu.Item {
	names := commandNames()
	skillNames := skillCommandNames(skills)
	items := make([]menu.Item, 0, len(names)+len(skillNames))
	for _, name := range names {
		items = append(items, menu.Item{Value: name})
	}
	for _, name := range skillNames {
		items = append(items, menu.Item{
			Value:       name,
			Description: skills[strings.TrimPrefix(name, "/skill:")].Description,
		})
	}
	return items
}

func (m *Model) refreshMenu() {
	word := menu.CommandWord(m.prompt.Value())
	m.prompt.SetStyles(m.promptStyles)
	if word == "" {
		m.menu.Reset()
	} else {
		m.menu.Filter(word)
	}
	m.layout(m.windowHeight)
}

func (m *Model) navigateMenu(press tea.KeyPressMsg) (bool, tea.Cmd) {
	switch press.String() {
	case "up":
		m.menu.Up()
	case "down":
		m.menu.Down()
	case "tab", "enter":
		m.selectMenuItem()
	case "esc":
		m.menu.Dismiss()
	default:
		return false, nil
	}
	m.menu.ClampView()
	m.layout(m.windowHeight)
	return true, nil
}

func (m *Model) selectMenuItem() {
	it, ok := m.menu.SelectedItem()
	if !ok {
		return
	}
	m.prompt.SetValue(menu.InsertPick(m.prompt.Value(), it.Value))
	m.prompt.CursorEnd()
	m.menu.Dismiss()
}

func (m *Model) viewMenu() string {
	return menu.View(&m.menu, max(m.width, 1), m.theme.Plain, m.theme.Menu, m.theme.Command)
}
