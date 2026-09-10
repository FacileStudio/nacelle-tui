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

// toolBorderMap pairs each built-in glyph with the lipgloss colour code its
// running box's left border wears: the basic-palette twin of the raw-escape
// code toolANSI keeps (34→4, 35→5, 36→6, 33→3). lipgloss.Color reads a bare
// number as an ANSI256 index, so an SGR code like "35" would render green
// where the glyph is magenta; the basic-palette form renders the exact hue the
// raw escape does. mcpANSI is a genuine ANSI256 code and needs no twin.
var toolBorderMap = map[string]string{
	"☰": "4",
	"◎": "4",
	"$": "5",
	"✎": "5",
	"✚": "5",
	"↧": "6",
	"≫": "3",
}

// ToolBorder returns the lipgloss colour code a tool's result box wears on its
// left border while the tool is still running: the tool's own glyph colour,
// orange (208) for an MCP tool. A finished box trades this for green or red
// via the caller.
func ToolBorder(name string, source nacelle.Source) string {
	if source == nacelle.ToolSourceMCP {
		return mcpANSI
	}
	if code, ok := toolBorderMap[ToolGlyph(name)]; ok {
		return code
	}
	return "4"
}
