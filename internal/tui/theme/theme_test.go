package theme

import (
	"strings"
	"testing"
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
