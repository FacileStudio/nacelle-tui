package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle-tui/internal/herdr"
	"github.com/FacileStudio/nacelle-tui/internal/sessions"
	"github.com/FacileStudio/nacelle-tui/internal/skills"
	"github.com/FacileStudio/nacelle-tui/internal/usage"
)

var delegations = make(chan nacelle.Usage, 64)

// DelegateUsage delivers a delegated run's spend to the update loop.
func DelegateUsage(u nacelle.Usage) {
	delegations <- u
}

// SessionConfig configures the runtime settings for an interactive session.
type SessionConfig struct {
	Root              string
	Model             string
	Backend           string
	Diffs             bool
	GroupTools        *bool
	ShowThinking      bool
	CompactAt         int64
	AutoResume        bool
	Resume            string
	PromptPrefix      string
	PromptPlaceholder string
	StartMessage      string
}

// UISession holds the complete state needed to run an interactive terminal session.
type UISession struct {
	Agent          *nacelle.Agent
	Banner         string
	Skills         []skills.Skill
	HookNotice     string
	Gate           *Approvals
	DelegateConfig nacelle.Config
	SessionConfig
}

// Launch starts the Bubble Tea UI session loop for the given configuration.
func Launch(c UISession) error {
	opened := NewModel(c.Agent, c.Banner, c.Skills, c.SessionConfig)
	opened.groupTools = c.GroupTools != nil && *c.GroupTools
	opened.Expanded = c.ShowThinking
	opened.run.root = c.Root
	opened.run.diffs = c.Diffs
	opened.delegate = c.DelegateConfig
	opened.sink = usage.NewSink(c.Root, c.Model)
	opened.session = sessions.OpenSession(c.Backend, c.Model, c.Root)
	herdr.SetSession(opened.herdrClient, opened.session.Path())
	if c.HookNotice != "" {
		opened.say(fromClient, c.HookNotice)
	}
	for _, line := range opened.unprinted {
		fmt.Println(line)
	}
	opened.unprinted = nil

	program := tea.NewProgram(opened)
	if c.Gate != nil {
		c.Gate.Wire(program.Send)
	}
	final, err := program.Run()

	if done, ok := final.(*Model); ok {
		herdr.Release(done.herdrClient)
		if recap := done.recap(); recap != "" {
			fmt.Println(recap)
		}
	}
	return err
}

func (m *Model) send(text string) tea.Cmd {
	m.run.stop = ""
	m.run.usage = nacelle.Usage{}
	m.run.liveOut = 0
	m.run.began = time.Now()
	m.run.turnBegan = time.Now()
	m.run.interrupted = time.Time{}
	m.run.asked, m.run.answered = nil, nil
	m.run.reported = false
	m.stranded()
	m.conversation = append(m.conversation, nacelle.UserText(text))

	ctx, cancel := context.WithCancel(context.Background())
	m.run.cancel = cancel
	m.run.bgCtx = ctx
	m.run.busy = true

	herdr.Report(m.herdrClient, herdr.Working)

	if count, err := m.agent.CountTokens(ctx, m.conversation); err == nil && m.compactAt > 0 && count > m.compactAt+compactSlack {
		m.size = count
		if waiting := m.beginCompaction(ctx); waiting != nil {
			return tea.Batch(waiting, m.spin.Tick)
		}
	}

	return m.startRun(ctx)
}

// startRun fires the model at the current conversation and returns the
// spinner-ticked wait a run is driven by. It is the one seam both send and
// settleCompaction reach: send starts a run directly, and a send that had to
// compact first hands control to the compaction's outcome, which calls this
// once the context is freed.
func (m *Model) startRun(ctx context.Context) tea.Cmd {
	m.run.results = start(ctx, m.agent, m.conversation)
	return tea.Batch(waitFor(m.run.results), m.spin.Tick)
}

func (m *Model) abandon() {
	m.run.interrupted = time.Now()
	m.run.stop = abandoned
	m.run.pending = nil
	m.run.cancel()
	if m.Len() > 0 {
		m.say(fromClient, fmt.Sprintf("%s dropped, not sent", countedNoun(m.Drop(), "queued message")))
		m.layout(m.windowHeight)
	}
}

func (m *Model) escaped() (bool, tea.Cmd) {
	if !m.run.busy {
		return false, nil
	}
	if time.Since(m.run.interrupted) >= forceQuit {
		m.run.interrupted = time.Now()
		m.run.stop = abandoned
		m.run.pending = nil
		m.run.cancel()
		if waiting := m.Len(); waiting > 0 {
			m.say(fromClient, "stopped · "+countedNoun(waiting, "queued message")+" still to send")
		}
	}
	return true, nil
}

func (m *Model) consume(next result) tea.Cmd {
	if next.err != nil {
		m.flush()
		m.run.reported = true
		m.say(fromFailure, next.err.Error())
		return waitFor(m.run.results)
	}
	m.record(next.event)
	m.absorb(next.event)
	return waitFor(m.run.results)
}

func (m *Model) recap() string {
	total := m.spent.Add(m.run.usage)
	tokens := total.InputTokens + total.OutputTokens + total.CacheReadTokens + total.CacheCreationTokens
	if m.tools == 0 && tokens == 0 {
		return ""
	}

	shape := "session · " + lasted(time.Since(m.began))
	switch {
	case m.tools == 1:
		shape += " · 1 tool"
	case m.tools > 1:
		shape += fmt.Sprintf(" · %d tools", m.tools)
	}
	if m.failed > 0 {
		shape += fmt.Sprintf(" · %d failed", m.failed)
	}

	spend := fmt.Sprintf("↑%s ↓%s",
		shortTokens(total.InputTokens+total.CacheCreationTokens),
		shortTokens(total.OutputTokens))
	if total.CacheReadTokens > 0 {
		spend += fmt.Sprintf(" · %s cached", shortTokens(total.CacheReadTokens))
	}
	if total.Cost > 0 {
		spend += fmt.Sprintf(" · $%.4f", total.Cost)
	}
	return shape + "\n" + spend
}
