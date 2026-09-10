package tui

import "fmt"

// countedNoun formats a count with its English plural, singular for exactly
// one.
func countedNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
