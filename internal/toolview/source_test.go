package toolview

import (
	"strings"
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

// ToolBorder returns the running state's one colour — yellow, basic "3" — for
// every tool, local or MCP alike, so a running box's spine always reads as
// "in flight". The finished box trades this for green or red via the caller.
func TestToolBorderIsAlwaysYellowWhileRunning(t *testing.T) {
	if got := ToolBorder(); got != "3" {
		t.Errorf("ToolBorder() = %q, want the running yellow 3", got)
	}
}

// ToolLineRunning paints a held line yellow, the same running-state colour the
// box spine wears, so a live row reads as "in flight" whatever its tool tone.
func TestToolLineRunningPaintsYellow(t *testing.T) {
	if got := ToolLineRunning("✎ edit_file(view.go)"); !strings.Contains(got, "\x1b[33m") {
		t.Errorf("ToolLineRunning = %q, want the SGR yellow 33 paint", got)
	}
}
