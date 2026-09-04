// Package status provides spinner components and status formatting helpers.
package status

import (
	"fmt"
	"strconv"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// WaitingPhrases is the sequence of waiting phrases.
var WaitingPhrases = []string{
	"waiting for a response",
	"waiting on the model",
	"waiting on the backend",
}

// Rephrase is the interval at which the waiting phrase rotates.
const Rephrase = 4 * time.Second

// WaitingVerb returns the waiting phrase for an elapsed duration.
func WaitingVerb(elapsed time.Duration) string {
	return WaitingPhrases[int(elapsed/Rephrase)%len(WaitingPhrases)]
}

// ShortTokens renders a token count formatted for display.
func ShortTokens(n int64) string {
	switch {
	case n >= 100_000_000:
		return fmt.Sprintf("%dM", n/1_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 100_000:
		return fmt.Sprintf("%dk", n/1_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return strconv.FormatInt(n, 10)
	}
}

// Lasted formats duration rounded to whole seconds, floored at one second.
func Lasted(spent time.Duration) string {
	return max(spent.Round(time.Second), time.Second).String()
}

// Took formats duration rounded to whole milliseconds, floored at one millisecond.
func Took(spent time.Duration) string {
	return max(spent.Round(time.Millisecond), time.Millisecond).String()
}

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
