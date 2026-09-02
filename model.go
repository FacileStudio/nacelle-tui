package main

import (
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"

	"github.com/FacileStudio/nacelle"
)

// forceQuit is how long the offer to quit outright stays open after a run is
// first asked to stop — long enough to read the status line, short enough that
// a ctrl+c minutes later still means "stop this run", not "quit".
const forceQuit = 3 * time.Second

// commandState is everything /skill:name and the dropdown menu need beyond
// what command.go itself owns: skills to resolve a name against, the
// dropdown's own filter/selection state, and the window height layout()
// needs to reserve the dropdown's own space out of. Embedded rather than
// named, the same reason Config embeds Discovery in config.go: every field
// still reads as m.skills or m.menu, not m.commandState.skills — grouping
// exists only to keep model's own field count from growing by one every
// time this list does.
type commandState struct {
	skills map[string]skill
	menu   commandMenu
}

// look is how a line is drawn rather than what it says: the palette resolved
// for the terminal's own background, the markdown renderer built for the
// current width, and the spinner that keeps the status line moving. All three
// are rebuilt or ticked by something other than the thing that produced the
// text, and none of them is ever read without the others nearby.
//
// Embedded, so every field still reads as m.theme, m.pretty and m.spin — the
// grouping exists for the same reason commandState's does, to keep model's own
// field count from growing by one every time this client learns to draw
// something new.
//
// spin is built with no style of its own and has to stay that way. The status
// line renders the spinner, the phrase and the clock as one coloured span, so
// a style here would emit its own reset in the middle of that span and drop
// the colour from everything after the spinner — see working().
type look struct {
	theme        palette
	pretty       *glamour.TermRenderer
	spin         spinner.Model
	groupTools   bool
	promptStyles textarea.Styles
}

// core groups the agent and the startup banner so model stays under filet's
// field cap. Embedded, so every field still reads as m.agent and m.banner.
type core struct {
	agent  *nacelle.Agent
	banner string
}

// transcriptSize groups the transcript-size settings so model stays under
// filet's field cap. Embedded, so every field still reads as m.compactAt
// and m.compacting.
type transcriptSize struct {
	compactAt  int64
	compacting bool
}

// model is the whole client: a transcript, a prompt, and at most one run in
// flight.
type model struct {
	core
	transcriptSize

	prompt    textarea.Model
	unprinted []string

	conversation []nacelle.Message
	account
	look
	commandState
	screen
	thoughts
	hist promptHistory
	run  inflight
}

// newModel builds the client. The banner names the backend and model, so
// which provider is billed is visible before typing, not after it fails.
// skills is every skill loaded this run — kept keyed by name so
// /skill:name is a lookup, not a scan, every time it's typed, and listed
// alongside the client's own commands in the dropdown menu.
func newModel(agent *nacelle.Agent, banner string, skills []skill, compactAt int64) *model {
	byName := bySkillName(skills)

	m := &model{
		core: core{agent: agent, banner: banner},
		transcriptSize: transcriptSize{compactAt: compactAt},
		prompt:    newPrompt(),
		look: look{
			theme: themed(true),
			spin:  spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		},
		account: account{began: time.Now()},
		screen:  screen{width: 80, liveRows: 1},
		commandState: commandState{
			skills: byName,
			menu:   commandMenu{items: menuItems(byName)},
		},
		run: inflight{
			runControl: runControl{cancel: func() {}},
			editState: editState{
				edits: map[string]editChange{}},
		},
	}
	m.pretty = prettier(m.theme.markdown, m.width)
	m.promptStyles = m.prompt.Styles()
	m.say(fromClient, banner+"\n")
	return m
}

// Init asks the terminal what colour it is, and opens the two watches that
// carry work in from goroutines this loop does not own — a delegated run's
// spend, and the plan the task tool reports.
//
// Both have to be armed here rather than only re-armed where they are handled.
// A watcher that is only re-armed by its own message never sees a first one, so
// the feature looks like it silently does nothing: the tool runs, returns
// happily to the model, and the screen never changes.
//
// There is no cursor-blink command because the prompt draws no cursor of its
// own — see newPrompt's SetVirtualCursor(false); the caret on screen is the
// terminal's, positioned by View, and a blink tick for a cursor nobody renders
// is a timer that wakes the program up to change nothing.
func (m *model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, watchDelegations(), watchTasks())
}

// Update routes each message to the one place that owns it, and hands whatever
// that had to say to the terminal before whatever it started.
//
// That ordering is the whole of a bug this client shipped on two paths at
// once. Two of the commands route returns block until the model sends
// something — waitFor, and the batch send wraps it in — and a sequence does
// not reach its next command until the one before it is done, so a line said
// on the way into a wait was not drawn until that wait ended. The question
// appeared when the answer did rather than when it was asked, and a tool's
// call line waited on the tool it announced: six seconds late for a
// six-second command, measured. Nothing already said is worth less than the
// thing it is waiting for. Fixing it here rather than in those two commands
// is the point — a third would arrive with no reason to know it had to flush
// first, which is how the second sat there while the first was being fixed.
//
// Two statements, not one call: as arguments they are evaluated left to right,
// which would drain the queue before the message that fills it was routed.
//
// Sequence, not Batch: a batch makes no promise about the order its commands
// run in, and the routed cmd may be tea.Quit — a quit that wins that race
// takes the last thing said with it, which for a queued /quit is the echo of
// the line that quit.
func (m *model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	started := m.route(message)
	return m, tea.Sequence(m.prints(), started)
}

// route is Update's own body, split out so that draining the print queue is
// one seam rather than a line repeated down every branch.
func (m *model) route(message tea.Msg) tea.Cmd {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		return m.resize(message)
	case tea.KeyPressMsg:
		if handled, cmd := m.key(message); handled {
			return cmd
		}
	case tea.BackgroundColorMsg:
		m.theme = themed(message.IsDark())
		m.restyle()
		return nil
	case spinner.TickMsg:
		return m.spun(message)
	case approvalRequest:
		m.run.pending = &message
		return nil
	case result:
		return m.consume(message)
	case finished:
		return m.settle()
	case spentDelegation:
		return m.recordDelegation(message)
	case taskUpdate:
		return m.recordTasks(message)
	}

	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(message)
	m.refreshMenu()
	return cmd
}
