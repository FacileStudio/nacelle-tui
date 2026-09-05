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

// Unstyled strips ANSI escape sequences from text.
func Unstyled(s string) string {
	return ansi.Strip(s)
}
