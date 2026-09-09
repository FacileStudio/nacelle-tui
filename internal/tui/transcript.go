package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle-tui/internal/sessions"
	"github.com/FacileStudio/nacelle-tui/internal/theme"
	"github.com/FacileStudio/nacelle-tui/internal/toolview"
)

// speaker is who a line on screen belongs to, which is all the drawing needs
// to know about it.
type speaker int

const (
	// fromClient is the client talking about itself: the banner, and what it
	// has to say about its own state.
	fromClient speaker = iota

	// fromReader is the question that was typed.
	fromReader

	// fromModel is the answer.
	fromModel

	// fromThinking is the model's reasoning.
	fromThinking

	// fromTool is a tool the model asked for.
	fromTool

	// fromResult is that tool having finished.
	fromResult

	// fromDiff is the before/after of a file edit, styled line by line by the
	// renderer that produced it rather than painted again here.
	fromDiff

	// fromFailure is the run falling over.
	fromFailure

	// fromTurn is the boundary line closing an assistant turn.
	fromTurn
)

// say commits one finished thing to the terminal's own scrollback.
//
// Nothing is kept. This client used to hold every line in a viewport and
// redraw the lot on each arriving character, which is what made it the only
// thing that could scroll them — and the reason the terminal could not. A
// finished line is printed once, above the live region, and belongs to the
// terminal from then on: its scrollback, its selection, its search, tmux's
// copy-mode. See View for why that is worth more than owning them.
//
// The line is queued rather than printed here because printing is a Cmd, and
// say has callers that cannot return one — absorb folding an event, flush
// committing an answer. Update drains the queue after every message, so the
// order lines are said in is the order they land in.
func (m *Model) say(who speaker, text string) {
	m.unprinted = append(m.unprinted, m.paint(who, text))
	m.session.Line(sessions.Speaker(who), text)
}

// prints hands everything said since the last message to the terminal, as a
// single Cmd, and forgets it.
//
// One Println for the batch rather than one per line: tea.Batch makes no
// promise about the order its commands run in, and a transcript delivered out
// of order is not a transcript. Joining them first makes the whole batch one
// message, which insertAbove writes in one go.
//
// One message, but not necessarily one Println — see printed, which cuts a
// batch taller than the window has room for into pieces that still arrive in
// order. Joining here and splitting there is deliberate: the split is a
// property of the screen, and nothing about what was said should have to know
// how tall the terminal is.
func (m *Model) prints() tea.Cmd {
	if len(m.unprinted) == 0 {
		return nil
	}
	said := strings.Join(m.unprinted, "\n")
	m.unprinted = nil
	return m.printed(said)
}

// paint is how one line looks.
//
// Nobody is labelled. A transcript prefixing every line with who said it
// spends the left margin on something the styling already says, and reads
// like a chat log rather than like a session. The reader's own question is
// the thing they scroll back to find, so that is what gets a muted background
// plus a bold pipe prefix; the answer is the thing being read, so it gets
// none, and is rendered as the markdown the model almost certainly wrote it
// in.
//
// What the client says about itself is the one thing not held to a width, and
// the banner is why. It is painted in newModel, before any WindowSizeMsg has
// arrived, so the only width available is the 80 the model starts at — and it
// is now printed to stdout before the program starts, so it is never repainted
// either. Held to 80 it wrapped "· bash on" onto a line of its own on every
// terminal wider than that. Unconstrained, the terminal wraps it the way it
// wraps everything else, which is what this file argues for everywhere else.
//
// Width is taken once, here, at the moment the line is printed. It can never
// be re-taken: the line is in the terminal's scrollback from then on, and a
// resize reflows it the way the terminal reflows everything else rather than
// the way this client would. That is the one thing owning a viewport bought
// that this gives up, and it is worth it — every other tool in the terminal
// behaves this way, including the shell the client was launched from.
func (m *Model) paint(who speaker, text string) string {
	width := max(m.width, 1)
	switch who {
	case fromReader:
		return m.theme.Question.Width(width).Render("| " + text)
	case fromModel:
		return m.markdown(text)
	case fromThinking:
		return m.theme.Thinking.Width(width).Render(text)
	case fromTool:
		return toolview.ToolLinePainted(text)
	case fromResult:
		return m.theme.Result.Width(width).Render("⤷ " + text)
	case fromDiff:
		return text
	case fromFailure:
		return m.theme.Failure.Width(width).Render(text)
	case fromTurn:
		return m.theme.Muted.Render(text)
	default:
		return m.theme.Client.Render(text)
	}
}

// streaming is what a run has produced but not finished: the reasoning and
// the answer as they arrive, drawn in the live region under everything
// already printed.
//
// It is tailed to the rows the window can spare rather than shown whole. The
// live region is repainted on every delta, so it has to fit on the screen —
// an answer longer than the terminal cannot be redrawn in place at all, and
// trying is how an inline program corrupts its own output. What scrolls off
// the top is not lost: the whole answer is printed, rendered, the moment it
// finishes.
//
// The answer is rendered through the markdown renderer live, because reading
// raw asterisks in the streaming region is worse than a slight reflow when a
// new character arrives. Half a code block falls back to plain text — glamour
// is lenient with incomplete markdown.
//
// It stamps the moment reasoning started on its way past. Drawing is not where
// a clock belongs, but this is the only thing that runs after every absorbed
// delta and lives in a file this may write to — absorb itself is in view.go.
// The frame is drawn after every message, so the stamp lands one frame after
// the first thinking delta, which is under a millisecond on a figure printed
// to a tenth of a second. See stamp.
func (m *Model) streaming() []string {
	var live []string
	if reasoning := m.run.reasoning.String(); reasoning != "" {
		m.Stamp()
		block := m.theme.Thinking.Render(m.Collapsed(m.Elapsed()))
		if m.Expanded {
			block = m.theme.Thinking.Width(max(m.width, 1)).Render(reasoning)
		}
		live = append(live, block)
	}

	if answer := m.run.answer.String(); answer != "" {
		live = append(live, m.markdown(answer))
	}
	groups := m.inFlightGroups()
	if len(groups) > 0 && len(live) > 0 {
		live = append(live, "")
	}
	live = append(live, groups...)
	if len(live) == 0 {
		return nil
	}

	return strings.Split(strings.Join(live, "\n"), "\n")
}

// inFlightGroups renders every tool group still running as a row the live
// region redraws each frame. A finished group is printed once and belongs to
// the terminal — only the still-open ones can grow.
func (m *Model) inFlightGroups() []string {
	var groups []string
	for _, g := range m.run.groups {
		if !g.End.IsZero() {
			continue
		}
		line := g.InFlightLine(m.width)
		if line == "" {
			continue
		}
		groups = append(groups, toolview.ToolLinePainted(line))
	}
	return groups
}

func (m *Model) restyle() {
	m.pretty = theme.Prettier(m.theme.Markdown, max(m.width, 1))
}

func (m *Model) markdown(text string) string {
	return theme.RenderMarkdown(m.pretty, text)
}

func countedNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
