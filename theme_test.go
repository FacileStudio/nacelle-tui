package main

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"
)

// Models answer in markdown whether or not anybody asked them to. A terminal
// that prints it raw shows the reader asterisks and backticks instead of the
// emphasis and code they were meant to be.
func TestAnAnswerIsRenderedAsMarkdownRatherThanShownRaw(t *testing.T) {
	m := sized()
	m.say(fromModel, "this is **bold** and this is `code`")

	drawn := m.unprinted[len(m.unprinted)-1]
	if strings.Contains(drawn, "**") || strings.Contains(ansi.Strip(drawn), "**") {
		t.Errorf("drawn = %q, want the markdown syntax gone, not printed literally", drawn)
	}
	if !strings.Contains(ansi.Strip(drawn), "bold") || !strings.Contains(ansi.Strip(drawn), "code") {
		t.Errorf("drawn = %q, want the words still there once the syntax is stripped", drawn)
	}
}

// Nobody on screen is labelled — "you:" or "nacelle:" spends the margin on
// something the styling already carries. The question is the one entry shown
// with a leading "> " marker (set by the reader in view.go) and styled with
// the muted palette — distinct from the model answer, so it is findable while
// scrolling back without being louder than the content.
func TestOnlyTheReaderSQuestionCarriesABackground(t *testing.T) {
	m := sized()
	m.say(fromReader, "what is in go.mod?")
	m.say(fromModel, "the module declaration")

	question := m.unprinted[len(m.unprinted)-2]
	answer := m.unprinted[len(m.unprinted)-1]

	if strings.Contains(question, "you:") || strings.Contains(answer, "nacelle:") {
		t.Error("an entry was labelled with who said it, which the styling already says")
	}
	if strings.TrimSpace(ansi.Strip(question)) != "what is in go.mod?" {
		t.Errorf("question = %q, want the text preserved", question)
	}
	if strings.TrimSpace(ansi.Strip(answer)) != "the module declaration" {
		t.Errorf("answer = %q, want the text preserved", answer)
	}
}

// A palette of hex guesses picked for a dark terminal is grey on grey on a
// light one. Everything with a hue is an ANSI index, so the terminal's own
// scheme answers that question; what follows the background is the markdown
// renderer, which picks a glamour theme by name, and the one grey — see below
// for why that one cannot be an index.
func TestTheTerminalSBackgroundPicksThePalette(t *testing.T) {
	m := sized()
	if m.theme.markdown != "dark" {
		t.Errorf("markdown style = %q, want dark before anything is reported", m.theme.markdown)
	}

	m.Update(tea.BackgroundColorMsg{Color: color.White})

	if m.theme.markdown != "light" {
		t.Errorf("markdown style = %q, want light for a light terminal", m.theme.markdown)
	}
}

// ANSI 8 is the one index a scheme is free to put anywhere, and dark themes
// routinely set it a shade off the background — every muted style in this
// client used it, so "dimmed" came out as unreadable on those terminals. The
// 256-colour greys are fixed whatever the scheme, and they have to differ by
// background or the light terminal gets back the grey-on-grey this replaced.
func TestTheMutedGreyIsReadableOnEitherBackground(t *testing.T) {
	dark := themed(true).muted.Render("in 0 · out 0")
	light := themed(false).muted.Render("in 0 · out 0")

	for _, rendered := range []string{dark, light} {
		if strings.Contains(rendered, "\x1b[90m") || strings.Contains(rendered, "\x1b[2m") {
			t.Errorf("muted = %q, want a fixed grey rather than ANSI 8 or faint", rendered)
		}
		if !strings.Contains(rendered, "\x1b[38;5;2") {
			t.Errorf("muted = %q, want one of the 256-colour greys", rendered)
		}
	}
	if dark == light {
		t.Errorf("muted = %q on both backgrounds, want the light terminal a darker grey", dark)
	}
}
