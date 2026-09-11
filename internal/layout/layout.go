// Package layout provides terminal dimensions, headroom, and width calculations.
package layout

import (
	"github.com/charmbracelet/x/ansi"
)

// Truncate fits s into limit terminal cells, ending with an ellipsis.
func Truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	return ansi.Truncate(s, limit, "…")
}

// Shave fits s into limit terminal cells with no marker, keeping the ANSI
// styling. Where Truncate announces the cut with an ellipsis, Shave just drops
// the overflow — the live-region margin clips a full-width row's trailing
// padding down to make room for its gutters, and a "…" there would read as
// content. Rows already within the limit pass through untouched.
func Shave(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	return ansi.Truncate(s, limit, "")
}

// Unstyled strips ANSI escape sequences from text.
func Unstyled(s string) string {
	return ansi.Strip(s)
}
