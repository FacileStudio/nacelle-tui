// Package tui implements the Bubble Tea terminal user interface.
package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle-tui/internal/history"
	"github.com/FacileStudio/nacelle-tui/internal/menu"
	"github.com/FacileStudio/nacelle-tui/internal/status"
	"github.com/FacileStudio/nacelle-tui/internal/theme"
)

// forceQuit is how long the offer to quit outright stays open after a run is
// first asked to stop — long enough to read the status line, short enough that
// a ctrl+c minutes later still means "stop this run", not "quit".
const forceQuit = 3 * time.Second

// NewModel builds the client. The banner names the backend and model, so
// which provider is billed is visible before typing, not after it fails.
// skills is every skill loaded this run — kept keyed by name so
// /skill:name is a lookup, not a scan, every time it's typed, and listed
// alongside the client's own commands in the dropdown menu.
func NewModel(agent *nacelle.Agent, banner string, skills []skill, compactAt int64, autoResume bool) *Model {
	byName := bySkillName(skills)

	m := &Model{
		core:       core{agent: agent, banner: banner, autoResume: autoResume},
		transcript: transcript{compactAt: compactAt},
		prompt:     newPrompt(),
		look: look{
			theme: theme.Themed(true),
			spin:  status.NewSpinner(),
		},
		account: account{began: time.Now()},
		screen:  screen{width: 80, liveRows: 1},
		commandState: commandState{
			skills: byName,
			menu:   *menu.New(menuItems(byName)),
		},
		run: inflight{
			runControl: runControl{cancel: func() {}},
			editState: editState{
				edits: map[string]editChange{}},
		},
		hist: history.New(),
	}
	m.pretty = theme.Prettier(m.theme.Markdown, m.width)
	m.promptStyles = m.prompt.Styles()
	m.say(fromClient, banner+"\n")
	return m
}

// Init asks the terminal what colour it is, and opens the watches that
// carry work in from goroutines this loop does not own — a delegated run's
// spend, the plan the task tool reports, and the titles the task-row
// summarizer returns.
//
// Both have to be armed here; a watcher only re-armed by its own message
// never sees a first run, so the feature silently does nothing.
//
// There is no cursor-blink command because the prompt draws no cursor of its
// own — see newPrompt's SetVirtualCursor(false); the caret on screen is the
// terminal's, positioned by View, and a blink tick for a cursor nobody renders
// is a timer that wakes the program up to change nothing.
func (m *Model) Init() tea.Cmd {
	if m.autoResume {
		projectRoot := m.run.root
		if projectRoot == "" {
			projectRoot = "."
		}

		sessionFiles := listSessionFiles(projectRoot)
		if len(sessionFiles) > 0 {
			mostRecent := sessionFiles[0]
			conversation := loadSession(mostRecent)
			if conversation != nil {
				m.conversation = conversation
				m.say(fromClient, fmt.Sprintf("resumed session from %s (%d messages)",
					filepath.Base(mostRecent), len(conversation)))
			}
		}
	}
	return tea.Batch(tea.RequestBackgroundColor, watchDelegations(), watchTasks(), watchTitles(), watchSides())
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
func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	started := m.route(message)
	return m, tea.Sequence(m.prints(), started)
}

// route is Update's own body, split out so draining the print queue is one seam.
// It is a tagged-union dispatcher: every message type hands off to its one
// owner and returns. The only arms that carry statements are the ones with
// real logic — a key the command palette did not consume, a palette switch,
// and the compaction-done branch; the rest are one-line forwards, and anything
// the switch does not match falls through to promptRoute. The frame is long
// because the dispatch is wide (13 arms plus the catch-all), not because any
// arm does much.
func (m *Model) route(message tea.Msg) tea.Cmd {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		return m.resize(message)
	case tea.KeyPressMsg:
		return m.keyOrPrompt(message)
	case tea.BackgroundColorMsg:
		m.theme = theme.Themed(message.IsDark())
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
	case taskTitled:
		return m.recordTitle(message)
	case sideResult:
		return m.recordSide(message)
	case taskUpdate:
		return m.recordTasks(message)
	case compactOutcome:
		return m.settleCompaction(message)
	case compactFinished:
		return m.finishCompaction()
	case tea.PasteMsg:
		return m.handlePaste(message)
	case tea.KeyboardEnhancementsMsg:
		return nil
	}

	return m.promptRoute(message)
}

// keyOrPrompt routes a handled key to the command palette, and sends one that
// was not consumed there on to the prompt — the route fall-through, folded into
// a call so the dispatcher stays one line per arm.
func (m *Model) keyOrPrompt(press tea.KeyPressMsg) tea.Cmd {
	if handled, cmd := m.key(press); handled {
		return cmd
	}
	return m.promptRoute(press)
}

// finishCompaction closes out a finished pass: the run that was holding the
// channel collapses, and the pending run resumes now that the context is free.
// Nothing queued means nothing waited on the pass, so there is no run to start.
func (m *Model) finishCompaction() tea.Cmd {
	m.compacting = false
	m.run.compactChan = nil
	if m.run.busy && m.agent != nil {
		return m.startRun(m.run.bgCtx)
	}
	return nil
}

// promptRoute forwards unhandled messages to the prompt and refreshes the dropdown.
func (m *Model) promptRoute(message tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(message)
	m.refreshMenu()
	return cmd
}

// handlePaste normalizes line endings and forwards pasted content to the prompt handler.
func (m *Model) handlePaste(msg tea.PasteMsg) tea.Cmd {
	msg.Content = strings.ReplaceAll(msg.Content, "\r\n", "\n")
	msg.Content = strings.ReplaceAll(msg.Content, "\r", "\n")
	return m.promptRoute(msg)
}
