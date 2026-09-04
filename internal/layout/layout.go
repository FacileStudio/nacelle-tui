// Package layout provides terminal dimensions, headroom, and width calculations.
package layout

import (
	"github.com/charmbracelet/x/ansi"
)

// Truncate fits s into max terminal cells, ending with an ellipsis.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	return ansi.Truncate(s, max, "…")
}

// Unstyled strips ANSI escape sequences from text.
func Unstyled(s string) string {
	return ansi.Strip(s)
}
