// Package settings resolves the linear precedence chain that turns flags, environment
// variables, a YAML file, and built-in defaults into the one Config the session acts on.
package settings

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v4"
)

// ConfigFile is where settings are read from when the flags leave them out.
const ConfigFile = ".nacelle.yml"

// Config is one layer of settings. Every field is a pointer or an empty-able
// string so that a layer can say nothing about a setting rather than saying
// zero, which is the whole difficulty of a precedence chain: "false" and "not
// mentioned" are different answers and a bool cannot tell them apart.
//
// It holds no credentials, deliberately. They already have two homes — the
// environment, and the Anthropic SDK's own profile — and a file with a key in
// it is a file that can never be committed to a dotfiles repo, which is the
// only reason to want one of these on two machines.
type Config struct {
	Backend string `yaml:"backend"`
	Model   string `yaml:"model"`
	Root    string `yaml:"root"`
	System  string `yaml:"system"`

	MaxIterations *int `yaml:"max_iterations"`

	Toggles `yaml:",inline"`

	Reasoning `yaml:",inline"`

	Web `yaml:",inline"`

	Discovery `yaml:",inline"`

	UI `yaml:",inline"`

	Sources `yaml:",inline"`

	Hooks []HookSpec `yaml:"hooks"`
}

// Toggles is the on/off settings: whether the model may run commands, whether
// a human is asked before tools run, and whether file edits are drawn as
// diffs. One field per key, all pointers, for the reason Config's own doc
// comment gives.
type Toggles struct {
	Bash         *bool `yaml:"bash"`
	Subagents    *bool `yaml:"subagents"`
	ApproveTools *bool `yaml:"approve_tools"`
	Diffs        *bool `yaml:"diffs"`
}

// UI holds the look-and-feel choices that are not about what the agent is
// allowed to do. GroupTools is on by default because a model that calls the
// same read ten times in a row is the common case, and ten lines of identical
// icon are noise; turning it off is for the session where you want to watch
// every call land.
//
// ShowThinking controls whether thinking traces are expanded by default. When
// true (the default), every turn's chain of thought is printed in full rather
// than collapsed to "thought for 2.9s". The ctrl+t key still toggles per-session
// either way, and show_thinking only sets the starting position.
type UI struct {
	GroupTools   *bool `yaml:"group_tools"`
	ShowThinking *bool `yaml:"show_thinking"`
}

// Reasoning holds the three settings that decide how hard the model thinks.
// Effort and Budget are two spellings of one idea — the backends disagree
// about which they accept, so each sends the one its own API understands.
type Reasoning struct {
	Effort   string `yaml:"effort"`
	Thinking *bool  `yaml:"thinking"`
	Budget   *int64 `yaml:"reasoning_budget"`
}

// Web holds the two network settings.
type Web struct {
	Search *string `yaml:"search"`
	Fetch  *bool   `yaml:"fetch"`
}

// Discovery holds the three settings that decide what this session folds into
// its system prompt from outside the conversation itself.
type Discovery struct {
	ProjectContext *bool `yaml:"project_context"`
	Skills         *bool `yaml:"skills"`
	TrustSkills    *bool `yaml:"trust_skills"`
	TrustHooks     *bool `yaml:"trust_hooks"`
}

// Sources holds the two settings that name somewhere to read from. SkillDirs
// replaces when set, MCP accumulates — the only list that does, because
// merging a config-file server list with a command-line server list is how
// both are used.
type Sources struct {
	SkillDirs []string `yaml:"skill_dirs"`
	MCP       []string `yaml:"mcp"`
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

// Defaults is the bottom layer, and the only one that answers everything.
func Defaults(system string) Config {
	bash, thinking, projectContext, skills, trustSkills, approveTools, trustHooks, diffs :=
		false, false, true, true, false, false, false, true
	subagents := false
	iterations, budget := 0, int64(0)
	search, fetch := "", true
	groupTools, showThinking := true, true
	return Config{
		Web:           Web{Search: &search, Fetch: &fetch},
		Backend:       "anthropic",
		Root:          ".",
		System:        system,
		Toggles:       Toggles{Bash: &bash, Subagents: &subagents, ApproveTools: &approveTools, Diffs: &diffs},
		MaxIterations: &iterations,
		Reasoning:     Reasoning{Thinking: &thinking, Budget: &budget},
		Discovery: Discovery{
			ProjectContext: &projectContext,
			Skills:         &skills,
			TrustSkills:    &trustSkills,
			TrustHooks:     &trustHooks,
		},
		UI: UI{GroupTools: &groupTools, ShowThinking: &showThinking},
	}
}

// ConfigPath is where the file lives, honouring HOME so a test does not have to
// write to the real one, and the empty string when there is no home to look in.
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
