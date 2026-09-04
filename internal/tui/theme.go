package tui

import "github.com/FacileStudio/nacelle-tui/internal/tui/theme"

func (m *Model) restyle() {
	m.pretty = theme.Prettier(m.theme.Markdown, max(m.width, 1))
}

func (m *Model) markdown(text string) string {
	return theme.RenderMarkdown(m.pretty, text)
}
