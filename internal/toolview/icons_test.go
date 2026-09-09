package toolview

import (
	"strings"
	"testing"
)

func TestToolKind(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"read_file", "read"},
		{"search_files", "read"},
		{"run_command", "write"},
		{"edit_file", "write"},
		{"write_file", "write"},
		{"web_fetch", "network"},
		{"subagent", "delegate"},
		{"parallel_subagent", "delegate"},
		{"unknown_custom_tool", "other"},
	}

	for _, tc := range cases {
		if got := ToolKind(tc.name); got != tc.want {
			t.Errorf("ToolKind(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestToolKindGlyph(t *testing.T) {
	cases := []struct {
		kind string
		want string
	}{
		{"read", "☰"},
		{"write", "$"},
		{"network", "↧"},
		{"delegate", "≫"},
		{"other", "•"},
	}

	for _, tc := range cases {
		if got := ToolKindGlyph(tc.kind); got != tc.want {
			t.Errorf("ToolKindGlyph(%q) = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestToolGlyph(t *testing.T) {
	if got := ToolGlyph("run_command"); got != "$" {
		t.Errorf("ToolGlyph(run_command) = %q, want $", got)
	}
	if got := ToolGlyph("unknown"); got != "•" {
		t.Errorf("ToolGlyph(unknown) = %q, want •", got)
	}
}

func TestToolRestore(t *testing.T) {
	if got := ToolRestore("run_command"); got != "35" {
		t.Errorf("ToolRestore(run_command) = %q, want 35", got)
	}
	if got := ToolRestore("unknown"); got != "34" {
		t.Errorf("ToolRestore(unknown) = %q, want 34", got)
	}
}

func TestColorGlyph(t *testing.T) {
	line := "$ run_command"
	colored := ColorGlyph(line, "32", "35")
	if !strings.Contains(colored, "\x1b[32m$\x1b[35m") {
		t.Errorf("ColorGlyph output %q missing expected escape codes", colored)
	}
}

func TestToolLinePainted(t *testing.T) {
	painted := ToolLinePainted("$ ls")
	if !strings.Contains(painted, "$ ls") {
		t.Errorf("ToolLinePainted = %q, want text included", painted)
	}
}
