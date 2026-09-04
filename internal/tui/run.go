package tui

import (
	"context"
	"fmt"
	"time"

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

// send starts a run over text.
func (m *Model) send(text string) tea.Cmd {
	m.run.stop = ""
	m.run.usage = nacelle.Usage{}
	m.run.began = time.Now()
	m.run.turnBegan = time.Now()
	m.run.interrupted = time.Time{}
	m.run.asked, m.run.answered = nil, nil
	m.run.reported = false
	m.stranded()
	m.conversation = append(m.conversation, nacelle.UserText(text))

	ctx, cancel := context.WithCancel(context.Background())
	m.run.cancel = cancel
	m.run.busy = true

	if count, err := m.agent.CountTokens(ctx, m.conversation); err == nil && m.compactAt > 0 && count > m.compactAt+compactSlack {
		m.size = count
		m.compact()
	}

	m.run.results = start(ctx, m.agent, m.conversation)
	return tea.Batch(waitFor(m.run.results), m.spin.Tick)
}

// halt stops the run in flight and leaves the queue standing.
func (m *Model) halt() {
	m.run.interrupted = time.Now()
	m.run.stop = abandoned
	m.run.pending = nil
	m.run.cancel()
}

func (m *Model) dropQueued() {
	if m.Len() == 0 {
		return
	}
	m.say(fromClient, fmt.Sprintf("%s dropped, not sent", countedNoun(m.Drop(), "queued message")))
	m.layout(m.windowHeight)
}

// abandon stops the run in flight and drops everything it would have led to.
func (m *Model) abandon() {
	m.halt()
	m.dropQueued()
}

// escaped is esc once the dropdown has had its turn.
func (m *Model) escaped() (bool, tea.Cmd) {
	if !m.run.busy {
		return false, nil
	}
	if time.Since(m.run.interrupted) >= forceQuit {
		m.halt()
		if waiting := m.Len(); waiting > 0 {
			m.say(fromClient, "stopped · "+countedNoun(waiting, "queued message")+" still to send")
		}
	}
	return true, nil
}

// consume folds one result into the transcript and waits for the next.
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
