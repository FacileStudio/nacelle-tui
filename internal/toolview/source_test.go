package toolview

import (
	"testing"

	"github.com/FacileStudio/nacelle"
)

func TestMCPSourceGlyph(t *testing.T) {
	if got := ToolSourceGlyph("whatever_mcp_tool", nacelle.ToolSourceMCP); got != "✻" {
		t.Errorf("ToolSourceGlyph(MCP) = %q, want ✻", got)
	}
	if got := ToolSourceGlyph("run_command", nacelle.ToolSourceLocal); got != "$" {
		t.Errorf("ToolSourceGlyph(local) = %q, want $", got)
	}
}

func TestMCPSourceRestore(t *testing.T) {
	if got := ToolSourceRestore("whatever_mcp_tool", nacelle.ToolSourceMCP); got != mcpANSI {
		t.Errorf("ToolSourceRestore(MCP) = %q, want %q", got, mcpANSI)
	}
	if got := ToolSourceRestore("_", nacelle.ToolSourceLocal); got != "34" {
		t.Errorf("ToolSourceRestore(unknown local) = %q, want 34", got)
	}
}

func TestMCPSourceColor(t *testing.T) {
	for _, ok := range []bool{true, false} {
		if got := ToolSourceColor("whatever_mcp_tool", nacelle.ToolSourceMCP, ok); got != mcpANSI {
			t.Errorf("ToolSourceColor(MCP, %v) = %q, want %q", ok, got, mcpANSI)
		}
	}
	if got := ToolSourceColor("run_command", nacelle.ToolSourceLocal, true); got != "32" {
		t.Errorf("ToolSourceColor(local ok) = %q, want 32", got)
	}
	if got := ToolSourceColor("run_command", nacelle.ToolSourceLocal, false); got != "31" {
		t.Errorf("ToolSourceColor(local fail) = %q, want 31", got)
	}
}

// ToolBorder returns the lipgloss basic-palette twin of the tool's raw glyph
// colour — same hue, different encoding — and keeps MCP's ANSI256 orange, so
// the running box's spine matches the glyph it sits under.
func TestToolBorderUsesTheGlyphsHueForLipgloss(t *testing.T) {
	if got := ToolBorder("whatever_mcp_tool", nacelle.ToolSourceMCP); got != mcpANSI {
		t.Errorf("ToolBorder(MCP) = %q, want %q", got, mcpANSI)
	}
	if got := ToolBorder("run_command", nacelle.ToolSourceLocal); got != "5" {
		t.Errorf("ToolBorder(run_command) = %q, want the basic-palette magenta 5", got)
	}
	if got := ToolBorder("edit_file", nacelle.ToolSourceLocal); got != "5" {
		t.Errorf("ToolBorder(edit_file) = %q, want the basic-palette magenta 5", got)
	}
	if got := ToolBorder("unknown", nacelle.ToolSourceLocal); got != "4" {
		t.Errorf("ToolBorder(unknown) = %q, want the plain-tool blue 4", got)
	}
}
