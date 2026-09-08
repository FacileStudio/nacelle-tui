package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle-tui/internal/toolview"
)

const abandoned nacelle.Stop = "abandoned"

func (m *Model) status() string {
	isReady := true
	state := "✓ ready"
	if cut := cutShort(m.run.stop); cut != "" {
		state = cut
		isReady = false
	}
	if m.run.busy {
		state = m.working()
		isReady = false
		if time.Since(m.run.interrupted) < forceQuit {
			state = "stopping · ctrl+c or ctrl+\\ to quit now"
		}
	}
	if m.run.pending != nil {
		state = fmt.Sprintf("approve %s(%s)? y = once · a = always this session · n = deny",
			m.run.pending.Name, truncate(unstyled(string(m.run.pending.Input)), 60))
	}

	if m.session != nil && m.session.HasWriteError() {
		state = "! could not write to session log · " + state
		isReady = false
	}

	width := max(m.width, 1)
	counts := strings.Join(m.footer(), " ")
	stateLine := truncate(state, width)
	if isReady {
		stateLine = m.theme.Ready.Render(stateLine)
	}
	return stateLine + "\n" + m.theme.Muted.Render(truncate(counts, width))
}

func (m *Model) footer() []string {
	total := m.spent.Add(m.run.usage)

	var spent []string
	if total.Cost > 0 {
		spent = append(spent, fmt.Sprintf("$%.4f", total.Cost))
	}
	spent = append(spent,
		"↑"+shortTokens(total.InputTokens+total.CacheCreationTokens),
		"↓"+shortTokens(total.OutputTokens))
	if m.size > 0 {
		spent = append(spent, "↕"+shortTokens(m.size))
	}
	return spent
}

func (m *Model) working() string {
	if m.compacting {
		return m.theme.Compacting.Render(m.spin.View() + " compacting context")
	}
	doing := waitingVerb(time.Since(m.run.began))
	tone := m.theme.Waiting
	switch n := m.running(); n {
	case 0:
	case 1:
		name, ok := m.runningName()
		if ok {
			doing = "running " + name
			tone = toolview.ToolTone(name)
		}
	default:
		doing = fmt.Sprintf("running %d tools", n)
		tone = m.theme.Tool
	}
	if since := m.ongoing(); since != "" {
		doing += " · " + since
	}
	return tone.Render(m.spin.View() + " " + doing)
}

func (m *Model) running() int {
	n := 0
	for _, g := range m.run.groups {
		if g.End.IsZero() {
			n++
		}
	}
	return n
}

func (m *Model) runningName() (string, bool) {
	for _, g := range m.run.groups {
		if g.End.IsZero() {
			return g.Tool.Name, true
		}
	}
	return "", false
}

func (m *Model) ongoing() string {
	if !m.run.busy || m.run.began.IsZero() {
		return ""
	}
	return lasted(time.Since(m.run.began))
}

func (m *Model) spun(message spinner.TickMsg) tea.Cmd {
	return m.spin.Spun(message, m.run.busy)
}

func cutShort(stop nacelle.Stop) string {
	if stop == "" || stop.Complete() {
		return ""
	}
	switch stop {
	case nacelle.StopMaxTokens:
		return "cut off at the token limit"
	case nacelle.StopContext:
		return "cut off: out of context"
	case nacelle.StopRefusal:
		return "refused by the model"
	case nacelle.StopIterations:
		return "stopped at the iteration limit"
	case abandoned:
		return "abandoned"
	}
	return "stopped early"
}
