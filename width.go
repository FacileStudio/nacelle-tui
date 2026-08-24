package main

import "github.com/charmbracelet/x/ansi"

// truncate fits s into max terminal cells, ending in an ellipsis when it had
// to cut something off.
//
// Cells, not bytes, and that is the difference between a guard and a bug. The
// byte-counting version this replaces was wrong twice over. It spent the whole
// budget and *then* appended its ellipsis, so the result came back one cell
// wider than it was asked for — enough to wrap the status line this is called
// on to keep to one row, which is the row layout() reserves exactly one of.
// And it cut on a byte boundary, so "aé" trimmed to two bytes produced invalid
// UTF-8: every queued line carrying an accent was one narrow window away from
// mojibake.
//
// ansi.Truncate is what knows the real answer, and the hand-rolled loop that
// used to live here got a third thing wrong that measuring in cells did not
// fix. It priced one rune at a time, and a single rune out of an escape
// sequence is not an escape sequence: every character of a "\x1b[38;5;245m"
// was charged a cell it does not occupy, so a coloured line was cut a dozen
// cells early and cut wherever the budget ran out — mid-sequence, dropping the
// reset, which leaks the colour into everything printed after it. Truncating a
// styled string is the normal case here, not the exotic one; the status line
// is styled before it gets this far.
func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	return ansi.Truncate(s, max, "…")
}

// unstyled strips the escape sequences out of text this client did not write,
// for the rows where it decides the styling itself.
//
// Everything measuring a line here skips ANSI rather than counting it, which
// is right for a line this client styled and wrong for one carrying somebody
// else's escapes: a tool argument ending in a bare "\x1b[7m" costs nothing,
// survives every width check, and leaves the terminal in reverse video for
// whatever is printed next. Tool inputs are written by the model and queued
// lines by whoever is typing, so neither is this client's to trust.
//
// Tool *output* is deliberately not run through this. A run_command printing
// its own colours is a command working correctly, and the transcript is where
// that belongs — the rule is about the rows this client owns and lays out, not
// about everything that crosses the screen.
func unstyled(s string) string { return ansi.Strip(s) }
