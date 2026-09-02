package settings

import (
	"flag"
	"strings"
)

// pathList collects one -skill-dir or -mcp flag per occurrence.
type pathList []string

func (p *pathList) String() string     { return strings.Join(*p, ":") }
func (p *pathList) Set(v string) error { *p = append(*p, v); return nil }

// declared holds every flag pointer returned by declareFlags.
type declared struct {
	backend, model, root, system *string
	reasoningFlags
	webFlags
	togglesFlags
	iterations *int
	// compactAt is a token count, not a turn count, so it shares the width of
	// budget rather than iterations.
	compactAt *int64
	sourceFlags
	discoveryFlags
}

type togglesFlags struct {
	bash, subagents, approveTools, diffs *bool
}

type reasoningFlags struct {
	effort   *string
	thinking *bool
	budget   *int64
}

type webFlags struct {
	search *string
	fetch  *bool
}

type sourceFlags struct {
	skillDirs, mcp *pathList
}

type discoveryFlags struct {
	projectContext, skills, trustSkills, trustHooks *bool
}

// declareFlags registers every flag against fallback's values and returns
// the pointers flag.Parse will fill in.
func declareFlags(fallback Config) declared {
	return declared{
		sourceFlags: declareSources(),
		backend:     flag.String("backend", fallback.Backend, "anthropic, google, openai, or openrouter"),
		model:       flag.String("model", fallback.Model, "model id, defaulting to the backend's own"),
		root:        flag.String("root", fallback.Root, "directory the file tools may reach"),
		system:      flag.String("system", fallback.System, "system prompt"),
		reasoningFlags: reasoningFlags{
			effort:   flag.String("effort", fallback.Effort, "none, minimal, low, medium, high, xhigh or max"),
			thinking: flag.Bool("thinking", *fallback.Thinking, "stream the model's reasoning"),
			budget:   flag.Int64("reasoning-budget", *fallback.Budget, "tokens one turn may spend on reasoning; 0 sets no ceiling"),
		},
		webFlags: webFlags{
			search: flag.String("search", *fallback.Search, "base URL of a SearXNG instance to search the web through; empty means no web search"),
			fetch:  flag.Bool("fetch", *fallback.Fetch, "let the model read a web page by URL; on by default"),
		},
		togglesFlags: togglesFlags{
			bash:         flag.Bool("bash", *fallback.Bash, "let the model run commands"),
			subagents:    flag.Bool("subagents", *fallback.Subagents, "give the model a subagent tool that delegates a self-contained task to a fresh nested run; off by default"),
			approveTools: flag.Bool("approve-tools", *fallback.ApproveTools, "ask before every tool call runs, y/a/n; off by default, every call runs unasked"),
			diffs:        flag.Bool("diffs", *fallback.Diffs, "show a git-style diff when the model edits a file; on by default"),
		},
		iterations: flag.Int("max-iterations", *fallback.MaxIterations, "how many times the model may be asked"),
		compactAt:  flag.Int64("compact-at", *fallback.CompactAt, "transcript size in tokens at which the session compacts; 0 turns compaction off"),
		discoveryFlags: discoveryFlags{
			projectContext: flag.Bool("project-context", *fallback.ProjectContext,
				"read CLAUDE.md and AGENTS.md from root upward into the system prompt"),
			skills: flag.Bool("skills", *fallback.Skills,
				"tell the model about skills found in ~/.agents/skills and trusted .agents/skills directories"),
			trustSkills: flag.Bool("trust-skills", *fallback.TrustSkills,
				"trust every .agents/skills directory found under root this run, and remember the decision"),
			trustHooks: flag.Bool("trust-hooks", *fallback.TrustHooks,
				"trust this project's .nacelle/hooks.yml as it reads right now, and remember that version"),
		},
	}
}

func declareSources() sourceFlags {
	skillDirs, mcp := new(pathList), new(pathList)
	flag.Var(skillDirs, "skill-dir",
		"extra directory to load skills from, alongside ~/.agents/skills (repeatable)")
	flag.Var(mcp, "mcp",
		"file of MCP servers to start and hand the model the tools of (repeatable)")
	return sourceFlags{skillDirs: skillDirs, mcp: mcp}
}

// typedSetters maps each flag's name to what it does to a Config.
func typedSetters(f declared) map[string]func(*Config) {
	return map[string]func(*Config){
		"backend":          func(c *Config) { c.Backend = *f.backend },
		"model":            func(c *Config) { c.Model = *f.model },
		"effort":           func(c *Config) { c.Effort = *f.effort },
		"root":             func(c *Config) { c.Root = *f.root },
		"system":           func(c *Config) { c.System = *f.system },
		"search":           func(c *Config) { c.Search = f.search },
		"fetch":            func(c *Config) { c.Fetch = f.fetch },
		"bash":             func(c *Config) { c.Bash = f.bash },
		"subagents":        func(c *Config) { c.Subagents = f.subagents },
		"thinking":         func(c *Config) { c.Thinking = f.thinking },
		"project-context":  func(c *Config) { c.ProjectContext = f.projectContext },
		"skills":           func(c *Config) { c.Skills = f.skills },
		"trust-skills":     func(c *Config) { c.TrustSkills = f.trustSkills },
		"trust-hooks":      func(c *Config) { c.TrustHooks = f.trustHooks },
		"approve-tools":    func(c *Config) { c.ApproveTools = f.approveTools },
		"diffs":            func(c *Config) { c.Diffs = f.diffs },
		"max-iterations":   func(c *Config) { c.MaxIterations = f.iterations },
		"compact-at":       func(c *Config) { c.CompactAt = f.compactAt },
		"reasoning-budget": func(c *Config) { c.Budget = f.budget },
		"skill-dir":        func(c *Config) { c.SkillDirs = []string(*f.skillDirs) },
		"mcp":              func(c *Config) { c.MCP = []string(*f.mcp) },
	}
}

// FromFlags is the settings layer the command line supplies.
//
// Only the flags actually typed are collected. It calls flag.Parse internally.
func FromFlags(fallback Config) Config {
	f := declareFlags(fallback)
	flag.Parse()
	typed := typedSetters(f)

	var flags Config
	flag.Visit(func(flg *flag.Flag) {
		if take, known := typed[flg.Name]; known {
			take(&flags)
		}
	})
	return flags
}
