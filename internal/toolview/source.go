package toolview

import (
	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle"
)

// ToolSourceGlyph returns the marker icon for a tool by its nacelle source.
// It is source-first: MCP tools get ❋ regardless of name. Everything else
// falls back to the normal name-based glyph.
func ToolSourceGlyph(name string, source nacelle.Source) string {
	if source == nacelle.ToolSourceMCP {
		return "❋"
	}
	return ToolGlyph(name)
}

// ToolSourceTone returns the Lipgloss style for a tool by its nacelle source.
func ToolSourceTone(name string, source nacelle.Source) lipgloss.Style {
	if source == nacelle.ToolSourceMCP {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	}
	return ToolTone(name)
}

// ToolSourceRestore returns the ANSI colour code for a tool by its nacelle source.
func ToolSourceRestore(name string, source nacelle.Source) string {
	if source == nacelle.ToolSourceMCP {
		return "35"
	}
	return ToolRestore(name)
}
