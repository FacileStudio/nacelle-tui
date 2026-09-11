// Package settings resolves the precedence chain that turns flags, environment
// variables, a YAML file, and built-in defaults into the one Config the session acts on.
package settings

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/FacileStudio/nacelle/mcp/client"
	"go.yaml.in/yaml/v4"
)

// ConfigFile is where settings are read from when the flags leave them out.
const ConfigFile = ".nacelle.yml"

// Limits is the threshold settings that cap the run. Embedded in Config so
// every field still reads as c.MaxIterations.
type Limits struct {
	MaxIterations *int   `yaml:"max_iterations"`
	CompactAt     *int64 `yaml:"compact_at"`
}

// Provider is the backend in use plus the endpoint and key that reach it.
// The yaml keys live under the group names (provider:, limits: and the rest);
// the NACELLE_ names are unchanged, so existing environments keep working.
type Provider struct {
	Backend string `yaml:"backend"`
	Model   string `yaml:"model"`
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

// Session is the launch settings that are not display choices — where the
// session starts, what its base prompt says, whether it resumes. They live
// under the session: group in the file, except resume: which is flag-only.
type Session struct {
	Root     string  `yaml:"root"`
	System   string  `yaml:"system_prompt"`
	Continue *bool   `yaml:"continue"`
	Resume   *string `yaml:"-"`
}

// NoConfig skips the file entirely; it comes from -no-config, never the
// file it is about to ignore.

// Config is one layer of settings. Every field is a pointer or an empty-able
// string so a layer can say nothing about a setting rather than saying zero:
// "false" and "not mentioned" are different answers and a bool cannot tell
// them apart. Its only credential is a custom endpoint's own api_key.
type Config struct {
	NoConfig *bool `yaml:"-"`

	Provider `yaml:"provider"`
	Session  `yaml:"session"`

	Limits `yaml:"limits"`

	Toggles `yaml:"tools"`

	Reasoning `yaml:"reasoning"`

	Security `yaml:"security"`

	Discovery `yaml:"discovery"`

	UI `yaml:"ui"`

	Sources `yaml:"sources"`
	Hooks   []HookSpec `yaml:"hooks"`
	Cron    []CronJob  `yaml:"cron"`
}

// Toggles is the tool-mount settings: whether the model gets each optional
// tool. Every toggle is a pointer so "not in this file" can be told from "false".
type Toggles struct {
	Bash           *bool `yaml:"run_command"`
	ParallelAgents *bool `yaml:"parallel_agents"`
	Fetch          *bool `yaml:"web_fetch"`
	Tasks          *bool `yaml:"tasks"`
}

// Security holds the two settings that decide how much a tool call may do
// before something stops it: ask before every call runs, confine to the root.
type Security struct {
	ApproveTools      *bool `yaml:"approve_tools"`
	StrictConfinement *bool `yaml:"strict_confinement"`
}

// UI holds display settings for the interactive client.
//
// Diffs shows a git-style diff when the model edits a file. GroupTools
// collapses consecutive read-only tool calls of one name into a single
// "running 10 tools" line while they run. ShowThinking expands thinking traces
// by default rather than collapsing to "thought for 2.9s"; ctrl+t toggles.
//
// PromptPlaceholder is the ghost text while the prompt is empty, StartMessage
// prints on launch above the banner, Mode is "inline" or "tui" rendering.
type UI struct {
	Mode              *string `yaml:"rendering_mode"`
	GroupTools        *bool   `yaml:"group_tools"`
	ShowThinking      *bool   `yaml:"show_thinking"`
	Diffs             *bool   `yaml:"diffs"`
	PromptPlaceholder *string `yaml:"prompt_placeholder"`
	StartMessage      *string `yaml:"start_message"`
	TransparentBlocks *bool   `yaml:"transparent_blocks"`
	JSON              *bool   `yaml:"cron_list_json"`
}

// Reasoning holds the three settings that decide how hard the model thinks.
// Effort and Budget spell one idea twice — the backends disagree on which
// they accept, so each sends the one its own API understands.
type Reasoning struct {
	Effort   string `yaml:"effort"`
	Thinking *bool  `yaml:"thinking"`
	Budget   *int64 `yaml:"budget"`
}

// Discovery holds the three settings that decide what this session folds into
// its system prompt from outside the conversation itself.
type Discovery struct {
	ProjectContext *bool `yaml:"project_context"`
	Skills         *bool `yaml:"skills"`
	TrustSkills    *bool `yaml:"trust_skills"`
	TrustHooks     *bool `yaml:"trust_hooks"`
}

// Sources names what to read. SkillDirs replaces when set. MCP holds servers
// written inline (the way most people configure one); MCPFiles names .mcp.json files to load as well
// (the -mcp flag), merged by server name with them win, so ~/.claude/.mcp.json keeps working.
type Sources struct {
	SkillDirs []string                    `yaml:"skill_dirs"`
	MCP       map[string]client.ServerDef `yaml:"mcp"`
	MCPFiles  []string                    `yaml:"-"`
}

// HookSpec is one entry under a config's `hooks:` key.
type HookSpec struct {
	On      string   `yaml:"on"`
	Match   []string `yaml:"match"`
	Run     string   `yaml:"run"`
	Timeout string   `yaml:"timeout"`
	Async   bool     `yaml:"async"`
}

// DerefBool reads a pointer out of a toggle. Every toggle is filled in by
// defaults, so the pointer is never nil by the time it reaches a caller.
func DerefBool(b *bool) bool {
	return b != nil && *b
}

// DefaultCompactAt is the transcript size, in tokens, at which a session
// with no opinion of its own compacts.
const DefaultCompactAt int64 = 75_000

// Defaults is the bottom layer, and the only one that answers everything.
func Defaults(system string) Config {
	bash, thinking, projectContext, skills, trustSkills, approveTools, trustHooks, diffs, tasks, strict :=
		true, true, true, true, false, false, false, true, true, false
	parallelAgents := true
	iterations, budget := 5, int64(0)
	compactAt := int64(75000)
	fetch := true
	groupTools, showThinking := true, true
	cont, resume := false, ""
	mode, transparent, json := "inline", false, false
	promptPlaceholder := "Ask something. Esc stops a run, ctrl+c stops or quits, ctrl+\\ forces it."
	startMessage := ""
	return Config{
		Provider:  Provider{Backend: "anthropic"},
		Session:   Session{Root: ".", System: system, Continue: &cont, Resume: &resume},
		Toggles:   Toggles{Bash: &bash, ParallelAgents: &parallelAgents, Fetch: &fetch, Tasks: &tasks},
		Security:  Security{ApproveTools: &approveTools, StrictConfinement: &strict},
		Limits:    Limits{MaxIterations: &iterations, CompactAt: &compactAt},
		Reasoning: Reasoning{Thinking: &thinking, Budget: &budget},
		Discovery: Discovery{
			ProjectContext: &projectContext,
			Skills:         &skills,
			TrustSkills:    &trustSkills,
			TrustHooks:     &trustHooks,
		},
		UI: UI{Mode: &mode, GroupTools: &groupTools, ShowThinking: &showThinking, Diffs: &diffs, PromptPlaceholder: &promptPlaceholder, StartMessage: &startMessage, TransparentBlocks: &transparent, JSON: &json},
	}
}

// ConfigPath is where the config file lives (HOME).
func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ConfigFile)
}

// Load reads the config file. A missing file is not an error — most people
// never write one — but an unreadable or malformed one is.
func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	var settings Config
	if err := decoder.Decode(&settings); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, &ParseError{Path: path, Err: err}
	}
	return settings, nil
}

// Settings resolves every layer in one place: flag beats environment beats
// file beats default. The scaffold runs before the file is read.
func Settings(system string, flags Config) (Config, error) {
	if flags.NoConfig != nil && *flags.NoConfig {
		resolved := Defaults(system)
		resolved.merge(FromEnv())
		resolved.merge(flags)
		return resolved, nil
	}
	if created, err := Scaffold(ConfigPath()); err != nil {
		return Config{}, err
	} else if created {
		fmt.Fprintln(os.Stderr, "wrote ~/.nacelle.yml with the default settings — edit it, or delete it to regenerate")
	}
	file, err := Load(ConfigPath())
	if err != nil {
		return Config{}, err
	}

	resolved := Defaults(system)
	resolved.merge(file)
	resolved.merge(FromEnv())
	resolved.merge(flags)
	return resolved, nil
}
