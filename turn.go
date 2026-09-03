package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/FacileStudio/nacelle"
)

func (m *model) turn(event nacelle.Event) {
	m.thought()
	m.commitTail()
	m.flushThinking()
	line := m.turnBoundary(event.Usage)
	m.say(fromTurn, line)
	m.run.usage = m.run.usage.Add(event.Usage)
	m.sink.record(event.Usage, time.Now())
	m.sized(event.Usage)
	m.compact()
	m.run.turnBegan = time.Time{}
}

func (m *model) commitTail() {
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

func (m *model) turnBoundary(usage nacelle.Usage) string {
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

func (m *model) flushThinking() {
	reasoning := m.run.reasoning.String()
	m.run.reasoning.Reset()

	if reasoning != "" || m.run.reasoningFull.Len() > 0 {
		spent := m.elapsed()
		m.begun, m.ended = time.Time{}, time.Time{}
		m.retained = m.run.reasoningFull.String() + reasoning
		m.run.reasoningFull.Reset()

		if m.expanded {
			if reasoning != "" {
				m.say(fromThinking, reasoning)
			}
		} else {
			m.say(fromThinking, m.collapsed(spent))
		}
	}
}
