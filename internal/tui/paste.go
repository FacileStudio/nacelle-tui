package tui

import (
	"strings"
	"unicode"
)

// sanitizePaste removes terminal control sequences from pasted content and
// normalizes line endings to Unix style. This keeps the prompt free from
// bracketed-paste wrappers, cursor movement sequences, and other embedded
// terminal control bytes that should not be treated as user input.
func sanitizePaste(s string) string {
	// Strip bracketed-paste mode wrappers. Some terminals leave these in
	// place if pasted content itself contains the wrapping bytes, so we
	// remove them defensively.
	s = strings.ReplaceAll(s, "\x1b[200~", "")
	s = strings.ReplaceAll(s, "\x1b[201~", "")

	// Replace carriage return / newline combinations with a single Unix
	// newline, then collapse standalone carriage returns.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Remove embedded terminal control sequences. This handles cursor
	// movement, kitty keyboard protocol sequences, and similar bytes
	// copied from terminal output.
	s = stripControlSequences(s)

	// Replace tabs with spaces. Tabs in pasted content are rarely wanted
	// in a prompt and make the stored text harder to read.
	s = strings.ReplaceAll(s, "\t", " ")

	// Drop any remaining control characters.
	return filterControls(s)
}

// stripControlSequences removes ANSI/DCS/OSC/KC sequences from text.
// The goal is narrow: remove terminal control bytes while preserving the
// readable content around them.
func stripControlSequences(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) {
			if s[i+1] == '[' {
				// CSI sequence: ESC [ <params> <final-byte>
				// Skip the ESC [ and consume parameter bytes until we
				// hit a letter in the final range.
				j := i + 2
				for j < len(s) {
					c := s[j]
					if (c >= '@' && c <= '~') || c == '\\' {
						break
					}
					j++
				}
				if j < len(s) {
					i = j + 1
				} else {
					i++
				}
				continue
			}
			if s[i+1] == ']' {
				// OSC sequence: ESC ] <params> BEL or ST
				j := i + 2
				for j < len(s) && s[j] != '\x07' {
					if j+1 < len(s) && s[j] == '\x1b' && s[j+1] == '\\' {
						break
					}
					j++
				}
				if j < len(s) {
					if s[j] == '\x07' {
						i = j + 1
					} else {
						i = j + 2
					}
				} else {
					i++
				}
				continue
			}
			if s[i+1] == 'P' {
				// DCS sequence: ESC P ... ST
				j := i + 2
				for j+1 < len(s) {
					if s[j] == '\x1b' && s[j+1] == '\\' {
						break
					}
					j++
				}
				if j+1 < len(s) {
					i = j + 2
				} else {
					i++
				}
				continue
			}
			// Other single-char escape sequences
			if i+2 < len(s) && s[i+2] == '\\' {
				i += 3
			} else {
				i += 2
			}
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

// filterControls removes remaining control characters after the targeted
// stripping above. This keeps the prompt content printable and safe to send
// to the model. Newlines are preserved as they are structural.
func filterControls(s string) string {
	var out strings.Builder
	for _, r := range s {
		if r == '\n' {
			out.WriteRune(r)
		} else if !unicode.IsControl(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}
