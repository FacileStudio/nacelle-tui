package tui

import (
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/glamour/v2"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle-tui/internal/history"
	"github.com/FacileStudio/nacelle-tui/internal/menu"
	"github.com/FacileStudio/nacelle-tui/internal/queue"
	"github.com/FacileStudio/nacelle-tui/internal/status"
	"github.com/FacileStudio/nacelle-tui/internal/theme"
)

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
	menu   menu.Menu
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
	theme        theme.Palette
	pretty       *glamour.TermRenderer
	spin         status.Spinner
	groupTools   bool
	promptStyles textarea.Styles
}

// core groups the agent and the startup banner so model stays under filet's
// field cap. Embedded, so every field still reads as m.agent and m.banner.
type core struct {
	agent      *nacelle.Agent
	banner     string
	autoResume bool
}

// transcript groups the conversation, unprinted lines, and transcript-size
// settings so model stays under filet's field cap. Every field still reads as
// m.conversation, m.unprinted, m.compactAt and m.compacting.
type transcript struct {
	conversation []nacelle.Message
	unprinted    []string
	compactAt    int64
	compacting   bool
}

// parallelTaskInfo holds info about a single task in a parallel subagent run.
type parallelTaskInfo struct {
	Task   string
	Result string
	Err    string
	Usage  nacelle.Usage
	Title  string
	Began  time.Time
	End    time.Time
	Active bool
}

// parallelState groups the parallel subagent UI state so model stays under
// filet's field cap. A map keyed by tool ID so that overlapping
// parallel_subagent calls do not overwrite each other — each call's tasks and
// result are tracked independently.
type parallelState struct {
	parallelTasks map[string][]parallelTaskInfo
}

// sideState groups the side-run list and the system prompt they launch with,
// so model stays under filet's field cap. Embedded, so every field still reads
// as m.sides and m.system.
type sideState struct {
	sides  []sideRun
	system string
}

// composer groups the prompt's own textarea and its recall history, so model
// stays under filet's field cap. Embedded, so every field still reads as
// m.prompt and m.hist.
type composer struct {
	prompt textarea.Model
	hist   *history.History
}

// Model is the whole client: a transcript, a prompt, and at most one run in
// flight.
type Model struct {
	core
	transcript
	queue.Queue

	composer

	account
	look
	commandState
	screen
	thoughts

	parallelState
	run inflight
	sideState
}
