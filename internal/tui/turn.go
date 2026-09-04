package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle-tui/internal/tasks"
)

func (m *Model) turn(event nacelle.Event) {
	m.Thought()
	m.commitTail()
	m.flushThinking()
	line := m.turnBoundary(event.Usage)
	m.say(fromTurn, line)
	m.run.usage = m.run.usage.Add(event.Usage)
	m.sink.Record(event.Usage, time.Now())
	m.sized(event.Usage)
	m.compact()
	m.run.turnBegan = time.Time{}
}

func (m *Model) commitTail() {
	m.commitParagraphs()
	if m.run.answer.Len() == 0 {
		return
	}
	tail := m.run.answer.String()
	m.run.committedLen += len(tail)
	m.run.answer.Reset()
	if tail != "" {
		m.say(fromModel, tail)
	}
}

func (m *Model) turnBoundary(usage nacelle.Usage) string {
	spent := time.Duration(0)
	if !m.run.turnBegan.IsZero() {
		spent = time.Since(m.run.turnBegan)
	} else if !m.run.began.IsZero() {
		spent = time.Since(m.run.began)
	}
	dur := "0s"
	if spent > 0 {
		dur = took(spent)
	}
	tokens := usage.Total()
	tokenStr := fmt.Sprintf("%s tokens", shortTokens(tokens))
	if tokens == 1 {
		tokenStr = "1 token"
	}
	pieces := []string{dur, tokenStr}
	if usage.Cost > 0 {
		pieces = append(pieces, fmt.Sprintf("$%.4f", usage.Cost))
	}
	return strings.Join(pieces, " · ")
}

func (m *Model) flushThinking() {
	reasoning := m.run.reasoning.String()
	m.run.reasoning.Reset()

	if reasoning != "" || m.run.reasoningFull.Len() > 0 {
		spent := m.Elapsed()
		m.Begun, m.Ended = time.Time{}, time.Time{}
		m.Retained = m.run.reasoningFull.String() + reasoning
		m.run.reasoningFull.Reset()

		if m.Expanded {
			if reasoning != "" {
				m.say(fromThinking, reasoning)
			}
		} else {
			m.say(fromThinking, m.Collapsed(spent))
		}
	}
}

func (m *Model) flush() string {
	m.flushThinking()

	full := m.run.fullAnswer.String()
	unprinted := ""
	if m.run.committedLen < len(full) {
		unprinted = full[m.run.committedLen:]
	}
	m.run.answer.Reset()
	m.run.fullAnswer.Reset()
	m.run.committedLen = 0
	if unprinted != "" {
		m.say(fromModel, unprinted)
	}
	return full
}

func (m *Model) reveal() (bool, tea.Cmd) {
	m.Expanded = !m.Expanded
	m.Hinted = true

	switch {
	case m.Expanded && m.Retained != "":
		m.say(fromThinking, m.Retained)
	case m.Expanded:
		m.say(fromClient, "reasoning will be shown in full from here")
	default:
		m.say(fromClient, "reasoning will collapse to a single line from here")
	}
	return true, nil
}

func watchTasks() tea.Cmd {
	return func() tea.Msg {
		return <-tasks.Reports
	}
}

func (m *Model) recordTasks(reported tasks.TaskUpdate) tea.Cmd {
	m.tasks = tasks.TaskList(reported)
	m.layout(m.windowHeight)
	return watchTasks()
}
