package main

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// toolIcons gives each local tool a marker glyph, chosen from characters that
// are text everywhere — geometric marks and arrows, nothing with an emoji
// presentation, so the transcript never depends on a colour font being
// installed. Tools outside the map, MCP ones especially, take the plain dot.
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

// toolStyles colours a call's line by its glyph, so every tool of one kind
// wears one colour at a glance. The palette stays inside the sixteen ANSI
// colours — red and green excepted, those belong to the diff renderer — so
// the shades follow the terminal's scheme like everything else here.
var toolStyles = map[string]lipgloss.Style{
	"$": lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
	"✎": lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
	"✚": lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	"☰": lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
	"◎": lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
	"↧": lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	"»": lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
}

// plainTool is what a tool with no glyph of its own wears — the blue the shell
// tool uses, which is the closest thing here to "a tool, unspecified".
//
// It is the single definition of that blue: toolLinePainted paints an unknown
// call's line with it, toolTone gives the status phrase the same one, and
// palette.tool takes it for "running N tools". Three places said Color("4")
// on their own before, which is three places to change and two to forget.
var plainTool = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))

// toolANSI is the ANSI foreground colour index for each glyph, used by
// colorGlyph in toolline.go to restore the tool colour after the outcome
// marker on the glyph. The mapping is what toolStyles applies, extracted to
// a form colorGlyph can insert directly: the number part of `\x1b[Nm`
// without the escape sequence itself.
var toolANSI = map[string]string{
	"$": "34",
	"✎": "35",
	"✚": "36",
	"☰": "33",
	"◎": "33",
	"↧": "36",
	"»": "35",
}

// plainToolANSI is the ANSI colour code for the plain tool fallback.
const plainToolANSI = "34"

// toolGlyph is the marker a tool's lines carry, and the key its colour is
// read from.
func toolGlyph(name string) string {
	if icon, ok := toolIcons[name]; ok {
		return icon
	}
	return "•"
}

// toolTone is the colour of whatever a named tool is doing right now, used by
// the status line while it waits on one.
//
// A tool with no glyph of its own — every MCP tool — falls back to the same
// blue toolLinePainted gives its line, not to no colour at all. Unstyled is
// the one answer this cannot return: the status line reads the phase off the
// colour now, so a plain tone would make an MCP call in flight look like a run
// that had stopped doing anything.
func toolTone(name string) lipgloss.Style {
	if style, ok := toolStyles[toolGlyph(name)]; ok {
		return style
	}
	return plainTool
}

// toolRestore returns the ANSI colour code to restore after the outcome
// marker on a tool's glyph, so the rest of the line keeps its tool colour.
// Unknown tools fall back to the plain tool blue.
func toolRestore(name string) string {
	if code, ok := toolANSI[toolGlyph(name)]; ok {
		return code
	}
	return plainToolANSI
}

// toolLinePainted colours a held call line by the glyph it opens with. A
// line without a known glyph falls back to the plain tool tone rather than
// staying unstyled — an unknown tool is still a tool worth seeing.
//
// An ANSI-prefixed text — one that already has a colour-glyph from colorGlyph
// in toolline.go — still gets wrapped in the tool hue: colorGlyph restores
// the tool colour after the glyph, so the outer style never overrides the
// outcome marker on the glyph itself.
func toolLinePainted(text string) string {
	glyph, _, _ := strings.Cut(text, " ")
	if len([]rune(glyph)) == 1 && glyph != "•" {
		if style, ok := toolStyles[glyph]; ok {
			return style.Render(text)
		}
	}
	return plainTool.Render(text)
}
