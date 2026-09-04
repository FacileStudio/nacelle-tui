package toolview

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle-tui/internal/theme"
)

var toolIcons = map[string]string{
	"run_command":  "$",
	"edit_file":    "✎",
	"write_file":   "✚",
	"read_file":    "☰",
	"search_files": "◎",
	"find_files":   "◎",
	"grep_files":   "◎",
	"web_fetch":    "↧",
	"download":     "↧",
	"subagent":     "»",
}

var toolStyles = map[string]lipgloss.Style{
	"☰": lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
	"◎": lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
	"$": lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
	"✎": lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
	"✚": lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
	"↧": lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	"»": lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
}

var toolANSI = map[string]string{
	"☰": "34",
	"◎": "34",
	"$": "35",
	"✎": "35",
	"✚": "35",
	"↧": "36",
	"»": "33",
}

// ToolKind categorizes a tool by its operation group for batching.
func ToolKind(name string) string {
	glyph := ToolGlyph(name)
	switch glyph {
	case "☰", "◎":
		return "read"
	case "$", "✎", "✚":
		return "write"
	case "↧":
		return "network"
	case "»":
		return "delegate"
	default:
		return "other"
	}
}

// ToolKindGlyph returns the representative icon for a tool kind.
func ToolKindGlyph(kind string) string {
	switch kind {
	case "read":
		return "☰"
	case "write":
		return "$"
	case "network":
		return "↧"
	case "delegate":
		return "»"
	default:
		return "•"
	}
}

// ToolGlyph returns the marker icon for a named tool.
func ToolGlyph(name string) string {
	if icon, ok := toolIcons[name]; ok {
		return icon
	}
	return "•"
}

// ToolTone returns the Lipgloss style for a named tool.
func ToolTone(name string) lipgloss.Style {
	if style, ok := toolStyles[ToolGlyph(name)]; ok {
		return style
	}
	return theme.PlainTool()
}

// ToolRestore returns the ANSI colour code to restore after a tool's glyph.
func ToolRestore(name string) string {
	if code, ok := toolANSI[ToolGlyph(name)]; ok {
		return code
	}
	return theme.PlainToolANSI
}

// ToolLinePainted colours a held call line by its opening glyph.
func ToolLinePainted(text string) string {
	glyph, _, _ := strings.Cut(text, " ")
	if len([]rune(glyph)) == 1 && glyph != "•" {
		if style, ok := toolStyles[glyph]; ok {
			return style.Render(text)
		}
	}
	return theme.PlainTool().Render(text)
}

// ColorGlyph colours the initial glyph of a line with color and restores restoreColour.
func ColorGlyph(line, color, restoreColour string) string {
	for i := 0; i < len(line); i++ {
		if line[i] == '\x1b' {
			end := strings.IndexByte(line[i:], 'm')
			if end < 0 {
				break
			}
			i += end
			continue
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		if r == utf8.RuneError {
			break
		}
		before := line[:i]
		glyph := line[i : i+size]
		rest := line[i+size:]
		return before + "\x1b[" + color + "m" + glyph + "\x1b[" + restoreColour + "m" + rest
	}
	return line
}
