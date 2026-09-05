package status

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// Spinner wraps a bubbletea spinner with run-aware ticking.
type Spinner struct {
	model spinner.Model
}

// NewSpinner returns an initialized spinner with MiniDot pattern.
func NewSpinner() Spinner {
	return Spinner{model: spinner.New(spinner.WithSpinner(spinner.MiniDot))}
}

// Tick returns the spinner tick message.
func (s Spinner) Tick() tea.Msg {
	return s.model.Tick()
}

// View renders the current spinner frame.
func (s *Spinner) View() string {
	return s.model.View()
}

// Spun advances the spinner one frame and returns the next tick if busy.
func (s *Spinner) Spun(msg spinner.TickMsg, busy bool) tea.Cmd {
	var cmd tea.Cmd
	s.model, cmd = s.model.Update(msg)
	if !busy {
		return nil
	}
	return cmd
}
