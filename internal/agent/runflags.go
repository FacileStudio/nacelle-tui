package agent

import (
	"fmt"
	"os"
	"time"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle/tools"

	"github.com/FacileStudio/nacelle-tui/internal/approval"
	"github.com/FacileStudio/nacelle-tui/internal/settings"
	"github.com/FacileStudio/nacelle-tui/internal/skills"
	"github.com/FacileStudio/nacelle-tui/internal/tui"
)

// Run is the main entry point for the agent.
func Run(v string) error {
	if handled, err := checkVersionFlag(v); handled {
		return err
	}
	if handled, err := checkCronFlag(); handled {
		return err
	}
	if handled, err := checkPrintFlag(); handled {
		return err
	}
	sess, cleanup, err := bootOrAsk(v)
	if err != nil {
		return err
	}
	defer cleanup()
	return tui.Launch(*sess)
}

type preparedTools struct {
	config settings.Config
	set    *tools.Set
	mcp    connected
	local  []nacelle.Tool
}

func setupAgentTools(noConfig bool) (preparedTools, error) {
	flags := settings.FromFlags(settings.Defaults(""))
	if noConfig {
		flags.NoConfig = &noConfig
	}
	config, err := settings.Settings(DefaultSystemPrompt(), flags)
	if err != nil {
		return preparedTools{}, err
	}
	set, local, err := localTools(config)
	if err != nil {
		return preparedTools{}, err
	}
	mcp, local, err := mcpTools(config, local)
	if err != nil {
		return preparedTools{}, closeOnErr(err, set)
	}
	return preparedTools{config: config, set: set, mcp: mcp, local: local}, nil
}

func setupAgentSession(p preparedTools, v string) (*tui.UISession, error) {
	found := augmentSystem(&p.config)
	approvalGate, approve := approval.Build(*p.config.ApproveTools)

	hooks, hookNotice, err := settings.SessionHooks(p.config)
	if err != nil {
		return nil, err
	}

	get, err := build(p.config, p.local, approve, hooks)
	if err != nil {
		return nil, err
	}

	return &tui.UISession{
		Agent:             get.agent,
		Banner:            banner(get.backend, p.config, found, p.mcp, v),
		Skills:            found.skills,
		HookNotice:        hookNotice,
		Gate:              approvalGate,
		DelegateConfig:    get.config,
		Mode:              *p.config.Mode,
		TransparentBlocks: *p.config.TransparentBlocks,
		SessionConfig: tui.SessionConfig{
			Root:              p.config.Root,
			Model:             p.config.Model,
			Backend:           p.config.Backend,
			Diffs:             *p.config.Diffs,
			GroupTools:        p.config.GroupTools,
			ShowThinking:      *p.config.ShowThinking,
			CompactAt:         resolveCompactAt(*p.config.CompactAt, get.backend),
			AutoResume:        *p.config.Continue,
			Resume:            *p.config.Resume,
			PromptPrefix:      *p.config.PromptPrefix,
			PromptPlaceholder: *p.config.PromptPlaceholder,
			StartMessage:      *p.config.StartMessage,
		},
	}, nil
}

func buildUISession(v string, noConfig bool) (*tui.UISession, func(), error) {
	prep, err := setupAgentTools(noConfig)
	if err != nil {
		return nil, nil, err
	}
	sess, err := setupAgentSession(prep, v)
	if err != nil {
		return nil, nil, closeOnErr(err, prep.set, prep.mcp.set)
	}
	return sess, func() {
		if err := closeAll(prep.set, prep.mcp.set); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}, nil
}

type loaded struct {
	notice       string
	skills       []skills.Skill
	contextFiles int
}

func augmentSystem(config *settings.Config) loaded {
	var found loaded
	config.System += environment(*config, time.Now())
	if *config.ProjectContext {
		text, files := projectContext(config.Root)
		config.System += text
		found.contextFiles = files
	}
	if !*config.Skills {
		return found
	}
	res := skills.LoadSkills(config.Root, *config.TrustSkills, config.SkillDirs)
	config.System += res.System
	found.notice = res.Notice
	found.skills = res.Skills
	return found
}
