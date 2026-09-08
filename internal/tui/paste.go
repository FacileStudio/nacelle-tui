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
	s = strings.ReplaceAll(s, "\x1b[200~", "")
	s = strings.ReplaceAll(s, "\x1b[201~", "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = stripControlSequences(s)
	s = strings.ReplaceAll(s, "\t", " ")
	return filterControls(s)
}

// stripControlSequences removes ANSI/DCS/OSC sequences from text.
// The goal is narrow: remove terminal control bytes while preserving the
// readable content around them.
func stripControlSequences(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) {
			i = skipControlSequence(s, i)
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

// skipControlSequence advances past one escape sequence starting at i.
func skipControlSequence(s string, i int) int {
	switch s[i+1] {
	case '[':
		return skipCSI(s, i)
	case ']':
		return skipOSC(s, i)
	case 'P':
		return skipDCS(s, i)
	}
	if i+2 < len(s) && s[i+2] == '\\' {
		return i + 3
	}
	return i + 2
}

// skipCSI advances past a CSI sequence starting at i (ESC [ ... final byte).
func skipCSI(s string, i int) int {
	j := i + 2
	for j < len(s) {
		c := s[j]
		if (c >= '@' && c <= '~') || c == '\\' {
			break
		}
		j++
	}
	if j < len(s) {
		return j + 1
	}
	return i + 1
}

// skipOSC advances past an OSC sequence starting at i (ESC ] ... BEL or ST).
func skipOSC(s string, i int) int {
	j := i + 2
	for j < len(s) && s[j] != '\x07' {
		if j+1 < len(s) && s[j] == '\x1b' && s[j+1] == '\\' {
			break
		}
		j++
	}
	if j < len(s) {
		if s[j] == '\x07' {
			return j + 1
		}
		return j + 2
	}
	return i + 1
}

// skipDCS advances past a DCS sequence starting at i (ESC P ... ST).
func skipDCS(s string, i int) int {
	j := i + 2
	for j+1 < len(s) {
		if s[j] == '\x1b' && s[j+1] == '\\' {
			break
		}
		j++
	}
	if j+1 < len(s) {
		return j + 2
	}
	return i + 1
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
