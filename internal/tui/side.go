package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle-tui/internal/layout"
	"github.com/FacileStudio/nacelle-tui/internal/status"
)

// A side run is a quick question answered by a fresh agent while the main run
// is busy — the concurrent path that stops a fan-out pinning the prompt. The
// parent agent is married to its parallel_subagent call, so the only way a
// message typed mid-run gets answered is a second, independent agent. That
// agent is isolated on purpose: it streams into the transcript so the reader
// sees the answer, but it never writes to m.conversation, so it cannot corrupt
// the parent's context or the session's ordering.

const sideMaxTokens int64 = 4000
const sideMaxIterations = 3

// sideRun is one in-progress or just-finished quick question. answer and
// committed track how much of the reply has reached the scrollback; usage is
// the run's own bill, shown on the row and the closing line.
type sideRun struct {
	prompt    string
	answer    strings.Builder
	committed int
	began     time.Time
	usage     nacelle.Usage
	done      bool
}

// sideResult is one event from a side run on the way to the update loop.
type sideResult struct {
	index int
	event nacelle.Event
	err   error
	done  bool
}

// sideResults is the channel every side run posts to, drained by watchSides.
var sideResults = make(chan sideResult, 256)

func watchSides() tea.Cmd {
	return func() tea.Msg {
		return <-sideResults
	}
}

// startSide launches a fresh agent to answer prompt while the main run is
// busy. The conversation it sees is a snapshot of everything committed so far
// plus the new question — m.conversation only changes at settle, so the copy
// taken here is a valid, re-sendable conversation and never races the parent.
func (m *Model) startSide(prompt string) {
	if m.agent == nil || m.system == "" {
		m.say(fromFailure, "cannot run a side answer: no agent is configured")
		return
	}
	backend := m.agent.Backend()
	sys := m.system
	index := len(m.sides)
	m.sides = append(m.sides, sideRun{prompt: prompt, began: time.Now()})
	conv := m.conversation[:]

	m.say(fromReader, prompt)
	m.layout(m.windowHeight)

	go func() {
		agent, err := nacelle.New(nacelle.Config{
			Backend:       backend,
			System:        sys,
			Thinking:      nacelle.Thinking{},
			MaxTokens:     sideMaxTokens,
			MaxIterations: sideMaxIterations,
		})
		if err != nil {
			sideResults <- sideResult{index: index, err: err, done: true}
			return
		}
		for event, err := range agent.Stream(context.Background(), append(conv, nacelle.UserText(prompt))) {
			sideResults <- sideResult{index: index, event: event, err: err}
			if err != nil {
				sideResults <- sideResult{index: index, done: true}
				return
			}
		}
		sideResults <- sideResult{index: index, done: true}
	}()
}

// recordSide applies one side event and re-arms the watch. Text commits to
// the scrollback in completed lines, exactly how the parent's own answer does,
// so a side reply streams instead of appearing as one block when the run ends.
func (m *Model) recordSide(n sideResult) tea.Cmd {
	if n.index >= len(m.sides) {
		return watchSides()
	}
	s := &m.sides[n.index]

	if n.err != nil && !s.done {
		s.done = true
		m.say(fromFailure, "side run failed: "+n.err.Error())
		m.layout(m.windowHeight)
	}
	if n.event.Kind == nacelle.KindText && !s.done {
		s.answer.WriteString(n.event.Text)
		m.commitSide(n.index)
	}
	if n.event.Kind == nacelle.KindTurn {
		s.usage = s.usage.Add(n.event.Usage)
	}
	if n.done && !s.done {
		s.done = true
		m.finishSide(n.index)
		m.layout(m.windowHeight)
	}
	return watchSides()
}

// commitSide moves a side reply's completed lines to the scrollback, keeping
// only the partial last line in the buffer. Same boundary as the parent's
// commitParagraphs, for the same reason: lines finish, paragraphs defer.
func (m *Model) commitSide(i int) {
	s := &m.sides[i]
	text := s.answer.String()
	idx := strings.LastIndex(text, "\n")
	if idx < 0 {
		return
	}
	complete := text[:idx]
	s.answer.Reset()
	s.answer.WriteString(text[idx+1:])
	s.committed += len(complete) + 1
	if complete != "" {
		m.say(fromModel, complete)
	}
}

// finishSide closes a side run: whatever text is still buffered is committed,
// and a boundary line names it as a side answer with its own duration and bill.
func (m *Model) finishSide(i int) {
	s := &m.sides[i]
	if tail := s.answer.String(); tail != "" {
		s.committed += len(tail)
		s.answer.Reset()
		m.say(fromModel, tail)
	}
	dur := status.Lasted(time.Since(s.began))
	m.say(fromTurn, fmt.Sprintf("side · %s · %s tokens", dur, status.ShortTokens(s.usage.Total())))
}

// dropFinishedSides forgets side runs whose answers already reached the
// transcript, so their rows leave at the next send or run end.
func (m *Model) dropFinishedSides() {
	kept := make([]sideRun, 0, len(m.sides))
	for _, s := range m.sides {
		if !s.done {
			kept = append(kept, s)
		}
	}
	m.sides = kept
}

// sideView is one line per side run under the prompt: the question collapsed
// to an action plus a live clock, flipping to a done check once the answer has
// committed. It never dumps the question in full, the way a task row no longer
// dumps a subagent's prompt.
func (m *Model) sideView() string {
	var lines []string
	for _, s := range m.sides {
		lines = append(lines, m.sideLine(s))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) sideLine(s sideRun) string {
	const gap = 3
	clock := status.Lasted(time.Since(s.began))
	if s.done {
		clock = "✓ " + clock
	}
	if s.usage.Total() > 0 {
		clock += " " + status.ShortTokens(s.usage.Total())
	}
	tone := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	if s.done {
		tone = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	}
	room := max(m.width-lipgloss.Width(clock)-gap, 0)
	head := tone.Render("⇄ " + layout.Truncate(strings.Join(strings.Fields(s.prompt), " "), room))
	return head + strings.Repeat(" ", max(m.width-lipgloss.Width(head)-lipgloss.Width(clock), 0)) + clock
}
