// Package cost tracks model usage pricing and calculates session costs.
package cost

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FacileStudio/nacelle"
)

// FormatTokens returns a compact token count representation.
func FormatTokens(n int64) string {
	if n < 1000 {
		return strconv.FormatInt(n, 10)
	}
	if n < 10_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%dk", n/1000)
}

// FormatCost formats a dollar amount to 4 decimal places.
func FormatCost(c float64) string {
	return fmt.Sprintf("$%.4f", c)
}

// FormatDuration formats an elapsed time duration floored at one second.
func FormatDuration(spent time.Duration) string {
	spent = spent.Truncate(time.Second)
	if spent < time.Second {
		return "1s"
	}
	return spent.String()
}

func countedNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// Summary builds the human-readable cost and token summary for a session.
func Summary(total nacelle.Usage, tools, failed int, duration time.Duration) string {
	pieces := []string{
		"session · " + FormatDuration(duration),
	}
	if tools > 0 {
		pieces = append(pieces, countedNoun(tools, "tool"))
	}
	if failed > 0 {
		pieces = append(pieces, fmt.Sprintf("%d failed", failed))
	}
	pieces = append(pieces,
		"in "+FormatTokens(total.InputTokens+total.CacheCreationTokens),
		"out "+FormatTokens(total.OutputTokens))
	if total.CacheReadTokens > 0 {
		pieces = append(pieces, FormatTokens(total.CacheReadTokens)+" cached")
	}
	if total.Cost > 0 {
		pieces = append(pieces, FormatCost(total.Cost))
	}
	return strings.Join(pieces, " · ")
}
