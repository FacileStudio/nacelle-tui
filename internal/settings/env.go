// Package settings resolves the linear precedence chain that turns flags, environment
// variables, a YAML file, and built-in defaults into the one Config the session acts on.
package settings

import (
	"os"
	"strconv"
	"strings"
)

// EnvPrefix is what every setting's environment variable starts with.
const EnvPrefix = "NACELLE_"

// FromEnv is the settings layer the environment supplies.
func FromEnv() Config {
	return Config{
		Backend: os.Getenv(EnvPrefix + "BACKEND"),
		Model:   os.Getenv(EnvPrefix + "MODEL"),
		Root:    os.Getenv(EnvPrefix + "ROOT"),
		System:  os.Getenv(EnvPrefix + "SYSTEM"),
		Limits:  Limits{MaxIterations: envInt(EnvPrefix + "MAX_ITERATIONS"), CompactAt: envInt64(EnvPrefix + "COMPACT_AT")},
		Sources: Sources{SkillDirs: envList(EnvPrefix + "SKILL_DIRS")},
		Toggles: Toggles{
			Bash:         envBool(EnvPrefix + "BASH"),
			Subagents:    envBool(EnvPrefix + "SUBAGENTS"),
			ApproveTools: envBool(EnvPrefix + "APPROVE_TOOLS"),
			Diffs:        envBool(EnvPrefix + "DIFFS"),
		},
		Reasoning: Reasoning{
			Effort:   os.Getenv(EnvPrefix + "EFFORT"),
			Thinking: envBool(EnvPrefix + "THINKING"),
			Budget:   envInt64(EnvPrefix + "REASONING_BUDGET"),
		},
		Web: Web{
			Search: envString(EnvPrefix + "SEARCH"),
			Fetch:  envBool(EnvPrefix + "FETCH"),
		},
		Discovery: Discovery{
			ProjectContext: envBool(EnvPrefix + "PROJECT_CONTEXT"),
			Skills:         envBool(EnvPrefix + "SKILLS"),
			TrustSkills:    envBool(EnvPrefix + "TRUST_SKILLS"),
		},
	}
}

// envBool reads a toggle, returning nil when the variable is unset or is not
// something strconv recognises.
func envBool(name string) *bool {
	raw, ok := os.LookupEnv(name)
	if !ok || raw == "" {
		return nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil
	}
	return &value
}

// envString reads a setting whose empty value means something, returning nil
// only when the variable is genuinely absent.
func envString(name string) *string {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return nil
	}
	return &raw
}

// envInt reads a count, with the same treatment of an unreadable value.
func envInt(name string) *int {
	raw, ok := os.LookupEnv(name)
	if !ok || raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &value
}

// envInt64 is envInt in the width a token count is measured in.
func envInt64(name string) *int64 {
	raw, ok := os.LookupEnv(name)
	if !ok || raw == "" {
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &value
}

// envList reads a colon-separated list, returning nil when unset or empty.
func envList(name string) []string {
	raw, ok := os.LookupEnv(name)
	if !ok || raw == "" {
		return nil
	}
	return strings.Split(raw, ":")
}
