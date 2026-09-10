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
