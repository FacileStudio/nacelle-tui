package agent

import (
	"fmt"
	"os"
	"strings"
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
	if handled, err := checkPrintFlag(); handled {
		return err
	}
	sess, cleanup, err := buildUISession(v)
	if err != nil {
		return err
	}
	defer cleanup()
	return tui.Launch(*sess)
}

func checkPrintFlag() (bool, error) {
	if !hasPrintFlag() {
		return false, nil
	}
	printArg := stripPrintFlag()
	if printArg == "" {
		piped, err := stdinPrompt()
		if err != nil {
			return true, fmt.Errorf("no prompt: neither -print nor stdin provided")
		}
		printArg = piped
	}
	return true, runHeadless(printArg)
}

type preparedTools struct {
	config settings.Config
	set    *tools.Set
	mcp    connected
	local  []nacelle.Tool
}

func setupAgentTools() (preparedTools, error) {
	flags := settings.FromFlags(settings.Defaults(""))
	config, err := settings.Settings("", flags)
	if err != nil {
		return preparedTools{}, err
	}
	set, local, err := localTools(config)
	if err != nil {
		return preparedTools{}, err
	}
	mcp, local, err := mcpTools(config, local)
	if err != nil {
		closeAll(set)
		return preparedTools{}, err
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

	agentInstance, backend, err := build(p.config, p.local, approve, hooks)
	if err != nil {
		return nil, err
	}

	return &tui.UISession{
		Agent:      agentInstance,
		Banner:     banner(backend, p.config, found, p.mcp, v),
		Skills:     found.skills,
		HookNotice: hookNotice,
		Gate:       approvalGate,
		SessionConfig: tui.SessionConfig{
			Root:         p.config.Root,
			Model:        p.config.Model,
			Backend:      p.config.Backend,
			Diffs:        *p.config.Diffs,
			GroupTools:   p.config.GroupTools,
			ShowThinking: *p.config.ShowThinking,
			CompactAt:    resolveCompactAt(*p.config.CompactAt, backend),
			AutoResume:   *p.config.Continue,
		},
	}, nil
}

func buildUISession(v string) (*tui.UISession, func(), error) {
	prep, err := setupAgentTools()
	if err != nil {
		return nil, nil, err
	}
	sess, err := setupAgentSession(prep, v)
	if err != nil {
		closeAll(prep.set, prep.mcp.set)
		return nil, nil, err
	}
	return sess, func() { closeAll(prep.set, prep.mcp.set) }, nil
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

func hasPrintFlag() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-print" || strings.HasPrefix(arg, "-print=") {
			return true
		}
	}
	return false
}

func stripPrintFlag() string {
	value := ""
	filtered := make([]string, 0, len(os.Args))
	filtered = append(filtered, os.Args[0])
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-print" {
			if i+1 < len(os.Args) && !strings.HasPrefix(os.Args[i+1], "-") {
				value = os.Args[i+1]
				i++
			}
		} else if strings.HasPrefix(arg, "-print=") {
			value = strings.TrimPrefix(arg, "-print=")
		} else {
			filtered = append(filtered, arg)
		}
	}
	os.Args = filtered
	return value
}
