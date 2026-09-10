package tui

import (
	"fmt"
	"strings"
)

// This file owns the wake-up that ends a detached fan-out: when every subagent
// has finished with real results while the main run is idle, the work is handed
// to the main agent so it reviews it. It is its own file because
// parallel_detached.go — where recordDetached lives — sits at filet's function
// cap, and the predicate and the message are worth testing on their own.

// parallelReview reports whether a completed fan-out should wake the main
// agent: nothing is still running, at least one task produced a real result,
// and the main run is idle. A fan-out that only failed leaves the review to the
// rows — nothing ran, so there is nothing to review. Cleared tasks (hidden by
// a /clear) are not counted, so a cleared fan-out never drags a stale result
// into a fresh session's review.
func (m *Model) parallelReview() bool {
	if m.run.busy || m.hasLiveParallel() {
		return false
	}
	for _, tasks := range m.parallelTasks {
		for _, pt := range tasks {
			if !pt.Cleared && pt.Result != "" {
				return true
			}
		}
	}
	return false
}

// reviewText builds the message that hands a completed fan-out's work to the
// main agent: one line per finished, un-cleared subagent, its title and what it
// produced, so the review the agent is asked for has the work in front of it.
func (m *Model) reviewText() string {
	var b strings.Builder
	b.WriteString("The parallel sub-agents have finished. Review their work and fold the results into your answer:\n")
	n := 0
	for _, tasks := range m.parallelTasks {
		for _, pt := range tasks {
			if pt.Cleared {
				continue
			}
			n++
			fmt.Fprintf(&b, "%d. %s\n", n, taskTitle(pt))
			body := pt.Result
			if pt.Err != "" {
				body = "errored: " + pt.Err
			}
			b.WriteString("   ")
			b.WriteString(strings.ReplaceAll(body, "\n", "\n   "))
			b.WriteString("\n")
		}
	}
	return b.String()
}
