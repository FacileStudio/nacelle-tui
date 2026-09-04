// Package thinking tracks reasoning state and formats thinking summaries.
package thinking

import (
	"fmt"
	"time"
)

const verbose = 10 * time.Second

// Thoughts tracks reasoning state across turns.
type Thoughts struct {
	Begun    time.Time
	Ended    time.Time
	Retained string
	Expanded bool
	Hinted   bool
}

// Stamp records when reasoning begins.
func (t *Thoughts) Stamp() {
	if t.Begun.IsZero() {
		t.Begun = time.Now()
	}
}

// Thought records when reasoning ended.
func (t *Thoughts) Thought() {
	if !t.Begun.IsZero() && t.Ended.IsZero() {
		t.Ended = time.Now()
	}
}

// Elapsed returns the duration spent thinking.
func (t *Thoughts) Elapsed() time.Duration {
	if t.Begun.IsZero() {
		return 0
	}
	if t.Ended.IsZero() {
		return time.Since(t.Begun)
	}
	return t.Ended.Sub(t.Begun)
}

// Forget clears retained reasoning and resets timestamps.
func (t *Thoughts) Forget() {
	t.Begun, t.Ended = time.Time{}, time.Time{}
	t.Retained = ""
}

// Collapsed formats a collapsed reasoning summary line.
func (t *Thoughts) Collapsed(spent time.Duration) string {
	hinted := t.Hinted
	t.Hinted = true
	return Collapsed(spent, hinted)
}

// Roughly formats a duration for display.
func Roughly(spent time.Duration) string {
	if spent >= verbose {
		return fmt.Sprintf("%ds", int(spent.Round(time.Second)/time.Second))
	}
	return fmt.Sprintf("%.1fs", spent.Round(100*time.Millisecond).Seconds())
}

// Collapsed formats the one line a turn's reasoning becomes.
func Collapsed(spent time.Duration, hinted bool) string {
	line := "\n▶ thought"
	if spent > 0 {
		line += " for " + Roughly(spent)
	}
	if !hinted {
		line += " · ctrl+t to expand"
	}
	return line
}
