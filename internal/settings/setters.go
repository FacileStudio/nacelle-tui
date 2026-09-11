package settings

import "maps"

// typedSetters maps each flag's name to what it does to a Config. The two
// halves exist because filet caps a function's statement count; the split
// follows the declared struct's own grouping.
func typedSetters(f declared) map[string]func(*Config) {
	setters := coreSetters(f)
	maps.Copy(setters, uiSetters(f))
	return setters
}

// uiSetters covers the flags declared in uiFlags: the session and display
// switches whose values are pointers straight off the command line.
func uiSetters(f declared) map[string]func(*Config) {
	return map[string]func(*Config){
		"continue":           func(c *Config) { c.Continue = f.cont },
		"resume":             func(c *Config) { c.Resume = f.resume },
		"mode":               func(c *Config) { c.Mode = f.mode },
		"transparent-blocks": func(c *Config) { c.TransparentBlocks = f.transparent },
		"json":               func(c *Config) { c.JSON = f.json },
		"no-config":          func(c *Config) { c.NoConfig = f.noConfig },
	}
}

func coreSetters(f declared) map[string]func(*Config) {
	return map[string]func(*Config){
		"backend":          func(c *Config) { c.Backend = *f.backend },
		"model":            func(c *Config) { c.Model = *f.model },
		"effort":           func(c *Config) { c.Effort = *f.effort },
		"root":             func(c *Config) { c.Root = *f.root },
		"system-prompt":    func(c *Config) { c.System = *f.system },
		"fetch":            func(c *Config) { c.Fetch = f.fetch },
		"bash":             func(c *Config) { c.Bash = f.bash },
		"parallel-agents":  func(c *Config) { c.ParallelAgents = f.parallelAgents },
		"thinking":         func(c *Config) { c.Thinking = f.thinking },
		"project-context":  func(c *Config) { c.ProjectContext = f.projectContext },
		"skills":           func(c *Config) { c.Skills = f.skills },
		"trust-skills":     func(c *Config) { c.TrustSkills = f.trustSkills },
		"trust-hooks":      func(c *Config) { c.TrustHooks = f.trustHooks },
		"approve-tools":    func(c *Config) { c.ApproveTools = f.approveTools },
		"diffs":            func(c *Config) { c.Diffs = f.diffs },
		"tasks":            func(c *Config) { c.Tasks = f.tasks },
		"max-iterations":   func(c *Config) { c.MaxIterations = f.iterations },
		"compact-at":       func(c *Config) { c.CompactAt = f.compactAt },
		"reasoning-budget": func(c *Config) { c.Budget = f.budget },
		"skill-dir":        func(c *Config) { c.SkillDirs = []string(*f.skillDirs) },
		"mcp":              func(c *Config) { c.MCPFiles = []string(*f.mcp) },
	}
}
