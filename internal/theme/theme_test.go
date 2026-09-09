package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestTheMutedGreyIsReadableOnEitherBackground(t *testing.T) {
	dark := Themed(true).Muted.Render("in 0 · out 0")
	light := Themed(false).Muted.Render("in 0 · out 0")

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

func TestRenderMarkdownNilFallback(t *testing.T) {
	raw := "plain text"
	if got := RenderMarkdown(nil, raw); got != raw {
		t.Errorf("RenderMarkdown(nil) = %q, want %q", got, raw)
	}
}

func TestAnAnswerIsRenderedAsMarkdownRatherThanShownRaw(t *testing.T) {
	r := Prettier("dark", 80)
	drawn := RenderMarkdown(r, "this is **bold** and this is `code`")
	if strings.Contains(drawn, "**") || strings.Contains(ansi.Strip(drawn), "**") {
		t.Errorf("drawn = %q, want markdown syntax rendered", drawn)
	}
	if !strings.Contains(ansi.Strip(drawn), "bold") || !strings.Contains(ansi.Strip(drawn), "code") {
		t.Errorf("drawn = %q, want words preserved", drawn)
	}
}

func TestOnlyTheReaderSQuestionCarriesABackground(t *testing.T) {
	p := Themed(true)
	question := p.Question.Render("| what is in go.mod?")
	answer := p.Plain.Render("the module declaration")
	if !strings.Contains(question, "what is in go.mod?") {
		t.Errorf("question = %q", question)
	}
	if !strings.Contains(answer, "the module declaration") {
		t.Errorf("answer = %q", answer)
	}
}

// Answers stream into the terminal one finished line at a time, each rendered
// on its own and joined back together with a single newline. Glamour frames
// every render with a leading newline and trailing fill, so without stripping
// that framing each joined line would be separated by a blank line.
func TestFragmentsJoinWithoutABlankLineBetweenThem(t *testing.T) {
	r := Prettier("dark", 80)
	a := RenderMarkdown(r, "first line of the answer")
	b := RenderMarkdown(r, "second line of the answer")
	joined := strings.Join([]string{a, b}, "\n")

	if count := strings.Count(joined, "\n\n"); count != 0 {
		t.Errorf("joined = %q, want no blank line between fragments, found %d", joined, count)
	}
	if strings.HasPrefix(joined, "\n") {
		t.Errorf("joined = %q, want no leading newline from glamour's top margin", joined)
	}
	if !strings.Contains(ansi.Strip(joined), "first line") || !strings.Contains(ansi.Strip(joined), "second line") {
		t.Errorf("joined = %q, want both lines preserved", joined)
	}
}

func TestTheTerminalSBackgroundPicksThePalette(t *testing.T) {
	dark := Themed(true)
	if dark.Markdown != "dark" {
		t.Errorf("markdown style = %q, want dark", dark.Markdown)
	}
	light := Themed(false)
	if light.Markdown != "light" {
		t.Errorf("markdown style = %q, want light", light.Markdown)
	}
}
