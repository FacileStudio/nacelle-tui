package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

type result struct {
	event nacelle.Event
	err   error
}

// Result carries an event or error from the agent stream.
type Result = result

// finished says the run's channel closed and no more results are coming.
type finished struct{}

// start runs an agent in its own goroutine and hands back the channel its
// results arrive on. The channel closes when the run ends, however it ends,
// which is what lets the update loop tell "still streaming" from "over".
func start(ctx context.Context, agent *nacelle.Agent, conversation []nacelle.Message) <-chan result {
	results := make(chan result)

	go func() {
		defer close(results)
		for event, err := range agent.Stream(ctx, conversation) {
			select {
			case results <- result{event: event, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	return results
}

// waitFor takes exactly one result and re-arms itself from Update.
func waitFor(results <-chan result) tea.Cmd {
	return func() tea.Msg {
		next, open := <-results
		if !open {
			return finished{}
		}
		return next
	}
}

func (m *Model) absorb(event nacelle.Event) {
	if m.run.turnBegan.IsZero() && event.Kind != nacelle.KindTurn && event.Kind != nacelle.KindDone && event.Kind != nacelle.KindToolResult {
		m.run.turnBegan = time.Now()
	}
	switch event.Kind {
	case nacelle.KindText:
		m.Thought()
		m.introduceReasoning()
		m.run.reported = m.run.reported || event.Text != ""
		m.run.answer.WriteString(event.Text)
		m.run.fullAnswer.WriteString(event.Text)
		m.commitParagraphs()
	case nacelle.KindThinking:
		m.run.reasoning.WriteString(event.Text)
		if m.Expanded {
			m.commitReasoning()
		}

	case nacelle.KindToolCall:
		m.absorbToolCall(*event.Tool)
	case nacelle.KindToolResult:
		m.absorbToolResult(*event.Tool, event.Tool.Result)
	case nacelle.KindTurn:
		m.turn(event)
	case nacelle.KindDone:
		m.run.usage = event.Usage
		m.run.stop = event.Stop
		m.sized(event.Usage)
	}
}

func (m *Model) absorbToolCall(tool nacelle.ToolEvent) {
	if tool.Name == "parallel_subagent" {
		m.rememberParallelCall(tool)
	}
	m.introduceReasoning()
	m.commitParagraphs()
	m.Thought()
	m.run.reported = true
	m.run.beginTool(tool, m.groupTools)
	if m.run.diffs {
		if change, ok := captureEdit(m.run.root, tool.Name, tool.Input); ok {
			m.run.edits[tool.ID] = change
		}
	}
}

func (m *Model) absorbToolResult(tool nacelle.ToolEvent, rawResult string) {
	if tool.Name == "parallel_subagent" {
		m.startDetachedParent(tool, rawResult)
	}
	m.run.finishTool(tool)
	m.finished(&tool)
}

func (m *Model) commitParagraphs() {
	text := m.run.answer.String()
	idx := strings.LastIndex(text, "\n")
	if idx < 0 {
		return
	}
	complete := text[:idx]
	partial := text[idx+1:]

	m.run.answer.Reset()
	m.run.answer.WriteString(partial)
	m.run.committedLen += len(complete) + 1

	if complete != "" {
		m.say(fromModel, complete)
	}
}

func (m *Model) commitReasoning() {
	text := m.run.reasoning.String()
	idx := strings.LastIndex(text, "\n")
	if idx < 0 {
		return
	}
	complete := text[:idx]
	partial := text[idx+1:]

	m.run.reasoning.Reset()
	m.run.reasoning.WriteString(partial)

	if complete != "" {
		m.run.reasoningFull.WriteString(complete)
		m.run.reasoningFull.WriteString("\n")
		m.say(fromThinking, complete)
	}
}
