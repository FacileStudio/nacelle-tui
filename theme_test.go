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
// something the styling already carries. The question is the one entry set in
// bold, which is how it is found again while scrolling back.
func TestOnlyTheReaderSQuestionCarriesABackground(t *testing.T) {
	m := sized()
	m.say(fromReader, "what is in go.mod?")
	m.say(fromModel, "the module declaration")

	question := m.unprinted[len(m.unprinted)-2]
	answer := m.unprinted[len(m.unprinted)-1]

	if !strings.Contains(question, "\x1b[1m") {
		t.Errorf("question = %q, want it bold so it is findable while scrolling back", question)
	}
	if strings.Contains(answer, "\x1b[1m") {
		t.Errorf("answer = %q, want it plain — the answer is what the window is for", answer)
	}
	if strings.Contains(question, "you:") || strings.Contains(answer, "nacelle:") {
		t.Error("an entry was labelled with who said it, which the styling already says")
	}
}

// A palette of hex guesses picked for a dark terminal is grey on grey on a
// light one. The palette is ANSI indices now, so the terminal's own scheme
// answers the colour question and both backgrounds render the same styles —
// what still changes with the background is the markdown renderer, which
// picks a glamour theme by name.
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
