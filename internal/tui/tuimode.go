package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// modeInline prints finished lines into the terminal's own scrollback and
// draws the live region and prompt beneath them, Bubble Tea's standard render.
// modeTUI holds the transcript inside its own buffer and draws it, live region,
// and prompt on an alternate screen with the prompt pinned to the bottom, like
// htop or vim. renderMode turns the config's mode string into one of them;
// anything but "tui" is inline.
const (
	modeInline int = 0
	modeTUI    int = 1
)

func renderMode(name string) int {
	if name == "tui" {
		return modeTUI
	}
	return modeInline
}

// assembleView builds the per-frame view. Inline mode reuses the terminal's own
// scrollback and renders only the live region, prompt, and menu. TUI mode
// assembles the whole screen — held transcript, live region, prompt pinned to
// the bottom — and marks it for the alternate screen.
func (m *Model) assembleView() tea.View {
	if m.mode == modeTUI {
		return m.assembleTUI()
	}
	return m.assembleInline()
}

// assembleTUI is the alternate-screen render. Every frame it builds the whole
// screen: the held transcript and live region tailed to whatever rows the screen
// can spare, and the prompt pinned to the bottom with exactly one blank row
// above and below, so it reads as a fixed input bar the way vim keeps its
// status line. There is no terminal scrollback to lean on inside an alternate
// screen, so finished lines are held in m.hold and drawn back here; when the
// transcript grows past the screen, the oldest rows scroll off rather than the
// prompt moving.
func (m *Model) assembleTUI() tea.View {
	prompt := m.prompt.View()
	below := m.belowContent()
	belowRows := 0
	if below != "" {
		belowRows = lipgloss.Height(below)
	}
	avail := max(m.windowHeight-lipgloss.Height(prompt)-2-belowRows, 1)
	parts := append(m.tuiUpper(avail), "", prompt)
	parts = append(parts, "")
	if below != "" {
		parts = append(parts, below)
	}
	body := strings.Join(parts, "\n")
	m.frameRows = lipgloss.Height(body)

	view := tea.NewView(body)
	view.AltScreen = true
	if position := m.prompt.Cursor(); position != nil {
		position.Y += avail + 1
		view.Cursor = position
	}
	return view
}

// tuiUpper is the scrolling region above the pinned prompt: the held transcript
// and the run's live output, newest at the bottom, padded to exactly avail rows
// so the prompt stays anchored. Each held entry is one pre-wrapped row — the
// split happened at print time — so joining the already-painted rows is enough;
// what the terminal clips horizontally, it clips. The hold is tailed to avail
// before the live rows join, since only the combined tail can reach the screen.
func (m *Model) tuiUpper(avail int) []string {
	held := m.hold
	if len(held) > avail {
		held = held[len(held)-avail:]
	}
	rows := make([]string, 0, avail)
	rows = append(rows, held...)
	for _, row := range m.aboveContent() {
		if row != "" {
			rows = append(rows, row)
		}
	}
	if len(rows) > avail {
		rows = rows[len(rows)-avail:]
	}
	var padded []string
	for pad := avail - len(rows); pad > 0; pad-- {
		padded = append(padded, "")
	}
	return append(padded, rows...)
}

// assembleInline is the scrollback render: everything already said lives in the
// terminal's own history, and the view draws only the live region, the prompt,
// and the menu beneath it. The prompt is separated from the content above and
// below by one blank row each, so it reads as its own band.
func (m *Model) assembleInline() tea.View {
	above := m.aboveContent()
	aboveHeight := lipgloss.Height(strings.Join(above, "\n"))
	parts := append(above, "", m.prompt.View())
	if below := m.belowContent(); below != "" {
		parts = append(parts, "", below)
	}
	body := strings.Join(parts, "\n")
	m.frameRows = lipgloss.Height(body)

	view := tea.NewView(body)
	if position := m.prompt.Cursor(); position != nil {
		position.Y += aboveHeight + 1
		view.Cursor = position
	}
	return view
}
