// Package settings resolves the linear precedence chain that turns flags, environment
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
// every field still reads as c.MaxIterations and c.CompactAt — the group
// exists only to keep the field count under filet's cap.
type Limits struct {
	MaxIterations *int   `yaml:"max_iterations"`
	CompactAt     *int64 `yaml:"compact_at"`
}

// Provider is the backend in use plus the endpoint and key that reach it:
// which vendor protocol, which model, which base URL, and the bearer key.
// Backend and Model were top-level settings until base_url and api_key joined
// them, and the four sit in one group only to stay under filet's struct cap.
// The yaml keys and NACELLE_ names are unchanged, so existing config files
// keep working.
type Provider struct {
	Backend string `yaml:"backend"`
	Model   string `yaml:"model"`
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

// Config is one layer of settings. Every field is a pointer or an empty-able
// string so that a layer can say nothing about a setting rather than saying
// zero, which is the whole difficulty of a precedence chain: "false" and "not
// mentioned" are different answers and a bool cannot tell them apart.
//
// The one credential it can carry is a custom endpoint's own api_key, where a
// dotfile is the reasonable home for it. A vendor key is still better kept in
// the environment: a file holding an actual OPENAI_API_KEY is a file that can
// never be committed to a dotfiles repo.
type Config struct {
	Provider `yaml:",inline"`
	Root     string `yaml:"root"`
	System   string `yaml:"system"`

	Limits `yaml:",inline"`

	Toggles `yaml:",inline"`

	Reasoning `yaml:",inline"`

	Web `yaml:",inline"`

	Discovery `yaml:",inline"`

	UI `yaml:",inline"`

	Sources `yaml:",inline"`

	Hooks []HookSpec `yaml:"hooks"`
}

// Toggles is the on/off settings: whether the model may run commands, whether
// the client will prompt for approval before a tool call runs, whether to show
// a diff when a file is changed, whether the model gets the parallel delegate
// tool, and whether the model is confined to the working directory. Every
// toggle is a pointer so "not in this file" can be told from "false".
type Toggles struct {
	Bash              *bool `yaml:"bash"`
	Subagents         *bool `yaml:"subagents"`
	ApproveTools      *bool `yaml:"approve_tools"`
	Diffs             *bool `yaml:"diffs"`
	Tasks             *bool `yaml:"tasks"`
	StrictConfinement *bool `yaml:"strict_confinement"`
}

// UI holds display settings for the interactive client.
//
// GroupTools controls whether consecutive read-only tool calls of the same
// name collapse into a single line while they run. When true (the default),
// ten search_content calls show as "running 10 tools" rather than taking ten
// lines of screen; the completed calls still each print their own line. It is
// on by default because twenty lines of the same tool name with the same cyan
// icon are noise; turning it off is for the session where you want to watch
// every call land.
//
// ShowThinking controls whether thinking traces are expanded by default. When
// true (the default), every turn's chain of thought is printed in full rather
// than collapsed to "thought for 2.9s". The ctrl+t key still toggles per-session
// either way, and show_thinking only sets the starting position.
//
// PromptPrefix names what the prompt's first row shows ahead of the caret, "| "
// by default; a wrapped question hangs its later rows under a matching indent.
// A single space of margin always follows the prefix, so the input never touches
// the left edge — an empty prefix still leaves one leading space.
// PromptPlaceholder is the ghost text the prompt shows while it is empty. StartMessage is printed as the first thing on
// launch, above the banner; it is a string that may span lines, so a welcome
// block or an ascii banner can sit there. Empty prints nothing.
type UI struct {
	Continue          *bool   `yaml:"continue"`
	GroupTools        *bool   `yaml:"group_tools"`
	ShowThinking      *bool   `yaml:"show_thinking"`
	PromptPrefix      *string `yaml:"prompt_prefix"`
	PromptPlaceholder *string `yaml:"prompt_placeholder"`
	StartMessage      *string `yaml:"start_message"`
}

// Reasoning holds the three settings that decide how hard the model thinks.
// Effort and Budget are two spellings of one idea — the backends disagree
// about which they accept, so each sends the one its own API understands.
type Reasoning struct {
	Effort   string `yaml:"effort"`
	Thinking *bool  `yaml:"thinking"`
	Budget   *int64 `yaml:"reasoning_budget"`
}

// Web holds the fetch setting, the one network tool that stays mounted.
type Web struct {
	Fetch *bool `yaml:"fetch"`
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
// with no opinion of its own compacts. It sits well inside the smallest
// window nacelle is aimed at, so the first sign of trouble is never
// StopContext: the floor it leaves below itself is room for a full answer
// plus the next turn's tools and system prompt.
const DefaultCompactAt int64 = 75_000

// Defaults is the bottom layer, and the only one that answers everything.
func Defaults(system string) Config {
	bash, thinking, projectContext, skills, trustSkills, approveTools, trustHooks, diffs, tasks, strict :=
		true, true, true, true, false, false, false, true, true, false
	subagents := true
	iterations, budget := 5, int64(0)
	compactAt := int64(75000)
	fetch := true
	groupTools, showThinking := true, true
	cont := false
	promptPrefix := "| "
	promptPlaceholder := "Ask something. Esc stops a run, ctrl+c stops or quits, ctrl+\\ forces it."
	startMessage := ""
	return Config{
		Web:       Web{Fetch: &fetch},
		Provider:  Provider{Backend: "anthropic"},
		Root:      ".",
		System:    system,
		Toggles:   Toggles{Bash: &bash, Subagents: &subagents, ApproveTools: &approveTools, Diffs: &diffs, Tasks: &tasks, StrictConfinement: &strict},
		Limits:    Limits{MaxIterations: &iterations, CompactAt: &compactAt},
		Reasoning: Reasoning{Thinking: &thinking, Budget: &budget},
		Discovery: Discovery{
			ProjectContext: &projectContext,
			Skills:         &skills,
			TrustSkills:    &trustSkills,
			TrustHooks:     &trustHooks,
		},
		UI: UI{Continue: &cont, GroupTools: &groupTools, ShowThinking: &showThinking, PromptPrefix: &promptPrefix, PromptPlaceholder: &promptPlaceholder, StartMessage: &startMessage},
	}
}

// ConfigPath is where the config file lives (HOME); a test need not touch the real one.
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
		return Config{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return settings, nil
}

// Settings resolves every layer in one place.
//
// Flag beats environment beats file beats default, and it is resolved here and
// nowhere else.
func Settings(system string, flags Config) (Config, error) {
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
