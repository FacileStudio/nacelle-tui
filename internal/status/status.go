// Package status provides spinner components and status formatting helpers.
package status

import (
	"fmt"
	"strconv"
	"time"
)

var waitingPhrases = []string{
	"waiting for a response",
	"waiting on the model",
	"waiting on the backend",
}

// WaitingPhrases returns a copy of the sequence of waiting phrases.
func WaitingPhrases() []string {
	return append([]string(nil), waitingPhrases...)
}

// Rephrase is the interval at which the waiting phrase rotates.
const Rephrase = 4 * time.Second

// WaitingVerb returns the waiting phrase for an elapsed duration.
func WaitingVerb(elapsed time.Duration) string {
	return waitingPhrases[int(elapsed/Rephrase)%len(waitingPhrases)]
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
