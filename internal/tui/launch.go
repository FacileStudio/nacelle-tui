package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/nacelle-tui/internal/sessions"
	"github.com/FacileStudio/nacelle-tui/internal/skills"
	"github.com/FacileStudio/nacelle-tui/internal/usage"
)

// SessionConfig groups runtime configuration options for the UI session.
type SessionConfig struct {
	Root         string
	Model        string
	Backend      string
	Diffs        bool
	GroupTools   *bool
	ShowThinking bool
	CompactAt    int64
	AutoResume   bool
}

// UISession is what the runner hands to the Bubble Tea program.
type UISession struct {
	Agent      *nacelle.Agent
	Banner     string
	Skills     []skills.Skill
	HookNotice string
	Gate       *Approvals
	SessionConfig
}

// Approvals represents the approval gate state.
type Approvals = approvals

// BuildApprovals constructs the approval gate and returns the approval function.
func BuildApprovals(config Config) (*Approvals, nacelle.Approve) {
	return buildApprovals(config)
}

// WireApprovals connects a non-nil gate to the running program's Send.
func WireApprovals(gate *Approvals, program *tea.Program) {
	wireApprovals(gate, program)
}

// Launch opens the program, delivers whatever was queued for the transcript
// before it opened, and prints the recap on exit.
func Launch(c UISession) error {
	opened := NewModel(c.Agent, c.Banner, c.Skills, c.CompactAt, c.AutoResume)
	opened.groupTools = derefBool(c.GroupTools)
	opened.expanded = c.ShowThinking
	opened.run.root = c.Root
	opened.run.diffs = c.Diffs
	opened.sink = usage.NewSink(c.Root, c.Model)
	opened.session = sessions.OpenSession(c.Backend, c.Model, c.Root)
	if c.HookNotice != "" {
		opened.say(fromClient, c.HookNotice)
	}
	for _, line := range opened.unprinted {
		fmt.Println(line)
	}
	opened.unprinted = nil

	program := tea.NewProgram(opened)
	WireApprovals(c.Gate, program)
	final, err := program.Run()

	if done, ok := final.(*Model); ok {
		if recap := done.recap(); recap != "" {
			fmt.Println(recap)
		}
	}
	return err
}

func derefBool(b *bool) bool {
	return b != nil && *b
}
