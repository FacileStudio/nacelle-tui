package toolview

import (
	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle"
)

// MCP tools get their own look: a ✻ glyph in orange, distinct from every
// built-in kind so a server's tools stand out in the transcript. The ANSI
// code and the ANSI-escape restore share the same value so the whole line
// reads orange, glyph and name together.
const mcpANSI = "208"

// ToolSourceGlyph returns the marker icon for a tool by its nacelle source.
// It is source-first: MCP tools get ✻ regardless of name. Everything else
// falls back to the normal name-based glyph.
func ToolSourceGlyph(name string, source nacelle.Source) string {
	if source == nacelle.ToolSourceMCP {
		return "✻"
	}
	return ToolGlyph(name)
}

// ToolSourceTone returns the Lipgloss style for a tool by its nacelle source.
func ToolSourceTone(name string, source nacelle.Source) lipgloss.Style {
	if source == nacelle.ToolSourceMCP {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(mcpANSI))
	}
	return ToolTone(name)
}

// ToolSourceRestore returns the ANSI colour code for a tool by its nacelle source.
func ToolSourceRestore(name string, source nacelle.Source) string {
	if source == nacelle.ToolSourceMCP {
		return mcpANSI
	}
	return ToolRestore(name)
}

// ToolSourceColor returns the ANSI colour code for a tool's leading glyph.
// A finished tool call marks its outcome on the glyph — green when it worked,
// red when it failed — but MCP keeps its own orange identity either way.
func ToolSourceColor(name string, source nacelle.Source, ok bool) string {
	if source == nacelle.ToolSourceMCP {
		return mcpANSI
	}
	if ok {
		return "32"
	}
	return "31"
}

// ToolBorder returns the lipgloss colour code a tool's box wears on its left
// border while the tool is still running: yellow (basic "3"), the running
// state's colour for every tool, local or MCP alike. A finished box trades
// this for green or red via the caller.
func ToolBorder() string {
	return "3"
}

// ToolLineRunning paints a held tool line yellow while its call is still
// running, the same running-state colour the box spine wears underneath. The
// finished report line keeps the tool's own tone via ToolLinePainted.
func ToolLineRunning(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render(text)
}
