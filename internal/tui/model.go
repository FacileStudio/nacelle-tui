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

// Init asks the terminal what colour it is, and opens the two watches that
// carry work in from goroutines this loop does not own — a delegated run's
// spend, and the plan the task tool reports.
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
func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	started := m.route(message)
	return m, tea.Sequence(m.prints(), started)
}

// route is Update's own body, split out so draining the print queue is one seam.
func (m *Model) route(message tea.Msg) tea.Cmd {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		return m.resize(message)
	case tea.KeyPressMsg:
		if handled, cmd := m.key(message); handled {
			return cmd
		}
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
	case taskUpdate:
		return m.recordTasks(message)
	case tea.PasteMsg:
		return m.handlePaste(message)
	}

	return m.promptRoute(message)
}

// promptRoute forwards unhandled messages to the prompt and refreshes the dropdown.
func (m *Model) promptRoute(message tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(message)
	m.refreshMenu()
	return cmd
}

// handlePaste sanitizes pasted content before inserting it into the prompt.
func (m *Model) handlePaste(msg tea.PasteMsg) tea.Cmd {
	clean := sanitizePaste(msg.Content)
	if clean == "" {
		return nil
	}

	m.prompt.InsertString(clean)
	m.refreshMenu()
	return nil
}

// parallelTasksView returns a view of the parallel subagent tasks.
func (m *Model) parallelTasksView() string {
	if len(m.parallelTasks) == 0 {
		return ""
	}
	var lines []string
	for _, tasks := range m.parallelTasks {
		lines = append(lines, callLines(tasks)...)
	}
	return strings.Join(lines, "\n")
}

// parallelTasksRows returns the number of parallel task rows for layout.
func (m *Model) parallelTasksRows() int {
	return parallelTaskRows(m.parallelTasks)
}
