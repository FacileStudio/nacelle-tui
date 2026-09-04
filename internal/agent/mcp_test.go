package agent

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

func namedServers(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".mcp.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing the MCP config: %v", err)
	}
	return path
}

func TestNoMCPConfigStartsNothingAndChangesNothing(t *testing.T) {
	local := []nacelle.Tool{}

	mcp, grown, err := mcpTools(defaults(), local)
	if err != nil {
		t.Fatalf("mcpTools: %v", err)
	}
	defer func() { _ = mcp.set.Close() }()

	if mcp.servers != 0 || mcp.tools != 0 {
		t.Errorf("connected = %+v, want nothing connected", mcp)
	}
	if len(grown) != 0 {
		t.Errorf("tools = %+v, want the local set handed straight back", grown)
	}
	if mcp.set == nil {
		t.Error("set = nil, want an empty Set the caller can close unconditionally")
	}
}

func TestTheBannerSaysNothingAboutMCPWhenNoneIsConfigured(t *testing.T) {
	got := testBanner(&answeringStub{}, asSettled(Config{Root: "."}), loaded{}, connected{})

	if strings.Contains(got, "MCP") {
		t.Errorf("banner = %q, want no mention of MCP when none is configured", got)
	}
}

func TestAServerThatWillNotStartEndsTheRun(t *testing.T) {
	config := defaults()
	config.MCP = []string{namedServers(t, `{"mcpServers": {"ledger": {"command": "/nonexistent/nacelle-mcp"}}}`)}

	_, _, err := mcpTools(config, nil)
	if err == nil {
		t.Fatal("a server that cannot start was accepted")
	}
	if !strings.Contains(err.Error(), "ledger") {
		t.Errorf("error = %v, want it to name the server that would not start", err)
	}
}

func TestAnUnreadableMCPFileEndsTheRun(t *testing.T) {
	config := defaults()
	config.MCP = []string{filepath.Join(t.TempDir(), "never-written.json")}

	if _, _, err := mcpTools(config, nil); err == nil {
		t.Fatal("a config file that was never there was accepted")
	}
}

func TestConfiguredExpandsTildesAndKeepsTheOrderGiven(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := configured([]string{"~/.claude/.mcp.json", "/etc/team.mcp.json"})

	want := []string{filepath.Join(home, ".claude", ".mcp.json"), "/etc/team.mcp.json"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("configured = %v, want %v", got, want)
	}
}

func TestTheBannerReportsTheServersAndToolsItWasGiven(t *testing.T) {
	got := testBanner(&answeringStub{}, asSettled(Config{Root: "."}), loaded{}, connected{servers: 2, tools: 7})

	if !strings.Contains(got, "2 MCP servers, 7 tools") {
		t.Errorf("banner = %q, want the server and tool counts", got)
	}
	if alone := testBanner(&answeringStub{}, asSettled(Config{Root: "."}), loaded{},
		connected{servers: 1, tools: 1}); !strings.Contains(alone, "1 MCP server, 1 tool") {
		t.Errorf("banner = %q, want both counts in the singular", alone)
	}
}

func TestMCPFilesFromTheFileAndTheFlagCombineWithTheFlagLast(t *testing.T) {
	written(t, "mcp:\n  - /from/the/file.json\n")

	config, err := resolveSettings(Config{Sources: Sources{MCP: []string{"/from/the/flag.json"}}})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if want := []string{"/from/the/file.json", "/from/the/flag.json"}; !slices.Equal(config.MCP, want) {
		t.Errorf("mcp = %v, want %v", config.MCP, want)
	}
}

func TestNoMCPKeyAnywhereLeavesTheListEmpty(t *testing.T) {
	written(t, "backend: openrouter\n")

	config, err := resolveSettings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if len(config.MCP) != 0 {
		t.Errorf("mcp = %v, want nothing configured", config.MCP)
	}
}
