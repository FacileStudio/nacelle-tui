package settings

// merge overwrites every setting the layer above actually mentions, and leaves
// the rest alone.
func (c *Config) merge(over Config) {
	c.mergeStrings(over)
	c.mergeToggles(over)
	c.mergeUI(over)
	if over.MaxIterations != nil {
		c.MaxIterations = over.MaxIterations
	}
	if over.CompactAt != nil {
		c.CompactAt = over.CompactAt
	}
	if over.Budget != nil {
		c.Budget = over.Budget
	}
	if over.Search != nil {
		c.Search = over.Search
	}
	if over.Fetch != nil {
		c.Fetch = over.Fetch
	}
	if len(over.SkillDirs) > 0 {
		c.SkillDirs = over.SkillDirs
	}
	c.MCP = append(c.MCP, over.MCP...)
	c.Hooks = append(c.Hooks, over.Hooks...)
}

// mergeStrings overwrites every string setting over actually mentions.
func (c *Config) mergeStrings(over Config) {
	if over.Backend != "" {
		c.Backend = over.Backend
	}
	if over.Model != "" {
		c.Model = over.Model
	}
	if over.Effort != "" {
		c.Effort = over.Effort
	}
	if over.Root != "" {
		c.Root = over.Root
	}
	if over.System != "" {
		c.System = over.System
	}
}

// mergeToggles overwrites every *bool setting over actually mentions.
func (c *Config) mergeToggles(over Config) {
	if over.Bash != nil {
		c.Bash = over.Bash
	}
	if over.Subagents != nil {
		c.Subagents = over.Subagents
	}
	if over.Thinking != nil {
		c.Thinking = over.Thinking
	}
	if over.ProjectContext != nil {
		c.ProjectContext = over.ProjectContext
	}
	if over.Skills != nil {
		c.Skills = over.Skills
	}
	if over.TrustSkills != nil {
		c.TrustSkills = over.TrustSkills
	}
	if over.TrustHooks != nil {
		c.TrustHooks = over.TrustHooks
	}
	if over.ApproveTools != nil {
		c.ApproveTools = over.ApproveTools
	}
	if over.Diffs != nil {
		c.Diffs = over.Diffs
	}
	if over.Tasks != nil {
		c.Tasks = over.Tasks
	}
}

// mergeUI overwrites every pointer field in UI that over actually mentions.
func (c *Config) mergeUI(over Config) {
	if over.GroupTools != nil {
		c.GroupTools = over.GroupTools
	}
	if over.ShowThinking != nil {
		c.ShowThinking = over.ShowThinking
	}
}
