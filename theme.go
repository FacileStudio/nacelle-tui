package main

import (
	"strings"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
)

// palette is every style the transcript uses, resolved for the terminal's own
// background rather than guessed at.
//
// Guessing is the usual way a terminal client ends up unreadable: a palette
// picked on somebody's dark terminal is grey on grey for everyone whose
// terminal is light. Bubble Tea can ask the terminal what colour it is and the
// answer arrives as a message, so the palette is rebuilt when it does and the
// dark one is only the assumption held until then.
type palette struct {
	question lipgloss.Style
	menu     lipgloss.Style
	command  lipgloss.Style
	thinking lipgloss.Style
	tool     lipgloss.Style
	result   lipgloss.Style
	failure  lipgloss.Style
	client   lipgloss.Style
	plain    lipgloss.Style
	waiting  lipgloss.Style
	muted    lipgloss.Style
	queued   lipgloss.Style
	markdown string
}

// themed builds the palette for a light or a dark terminal.
//
// The question is the only thing given a background, and it is a quiet one: it
// exists so the reader can find where they asked something while scrolling back
// through an answer, not to decorate. Everything that is not the answer is
// dimmed instead, because the answer is what the window is for.
//
// Bold alone was not enough to find. An answer is full of bold — every heading
// and every emphasised phrase the model writes — so the one bold line that
// means "you said this" was competing with a page of them, and scrolling back
// for your own question meant reading rather than glancing. The background is
// the menu's, deliberately: the two rows this client owns and the reader acts
// on should look like each other, and paint pads the row to the full width so
// the bar is the shape of a bar rather than of the sentence.
//
// Dimmed is not the same as unreadable, and the difference is which grey. Every
// muted style here used ANSI 8, which is the one index a scheme is free to put
// wherever it likes — dark themes routinely set it a shade or two off the
// background, so "dimmed" came out as barely-there on the terminals that do.
// The 256-colour ramp is not themeable: 243 and 245 are #767676 and #8a8a8a
// whatever the scheme, which clears 4.5:1 against white and black respectively,
// so the muted text stays muted and stays legible. Everything with a hue is
// still an ANSI index, because a hue is exactly what a scheme should own.
//
// waiting is the one muted-looking thing that is not muted. It was the same
// grey as everything else, on the row that is the only proof the client is
// still doing something, which left the busiest line on screen looking like
// the deadest one. Cyan reads as pending and is not blue, so the phrase
// changing colour is itself the signal that waiting turned into running.
func themed(dark bool) palette {
	pick := lipgloss.LightDark(dark)
	quiet := pick(lipgloss.Color("242"), lipgloss.Color("244"))
	faint := pick(lipgloss.Color("236"), lipgloss.Color("253"))
	faintFg := pick(lipgloss.Color("15"), lipgloss.Color("0"))

	style := "light"
	if dark {
		style = "dark"
	}

	return palette{
		question: lipgloss.NewStyle().
			PaddingLeft(1).
			Background(faint).
			Foreground(faintFg),
		menu: lipgloss.NewStyle().
			Background(pick(lipgloss.Color("7"), lipgloss.Color("8"))).
			Foreground(pick(lipgloss.Color("0"), lipgloss.Color("7"))),
		command:  lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
		thinking: lipgloss.NewStyle().Foreground(quiet).Italic(true),
		tool:     plainTool,
		result:   lipgloss.NewStyle().Foreground(quiet),
		failure:  lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		client:   lipgloss.NewStyle().Foreground(quiet),
		plain:    lipgloss.NewStyle(),
		waiting:  lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
		queued: lipgloss.NewStyle().
			Background(faint).
			Foreground(faintFg),
		muted:    lipgloss.NewStyle().Foreground(quiet),
		markdown: style,
	}
}

// restyle rebuilds everything that depends on the width or the terminal's
// colour, and draws the transcript again with it.
func (m *model) restyle() {
	m.pretty = prettier(m.theme.markdown, max(m.width, 1))
}

// prettier builds the markdown renderer for one width and one terminal, or nil
// when it cannot be built. Nothing about an answer is worth failing to start
// over, and markdown renders as itself when it is not rendered at all.
func prettier(style string, width int) *glamour.TermRenderer {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	return renderer
}

// markdown renders an answer the way the model wrote it.
//
// Models answer in markdown whether or not anybody asked them to, so a terminal
// printing it raw shows the reader asterisks and backticks wrapped around the
// thing they wanted to read. Text the renderer cannot parse falls back to
// itself: an unreadable answer is worse than an unstyled one, and the model is
// perfectly capable of emitting markdown that is not quite markdown.
//
// The trailing newlines go because the renderer pads a document to sit alone on
// a page, and this one sits in a transcript that spaces its own entries.
func (m *model) markdown(text string) string {
	if m.pretty == nil {
		return text
	}
	rendered, err := m.pretty.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimRight(rendered, "\n")
}
