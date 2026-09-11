package tui

import (
	"context"
	"strings"
	"testing"
)

// A run cancelled in-flight — the reader stopping it, or a parallel fan-out
// relaxing the parent — sees the stream's trailing context.Canceled as its last
// error. That is a normal stop, never a failure, so consume must not paint a red
// "context canceled" line over an otherwise clean end.
func TestACancelledRunDoesNotFlashAFailure(t *testing.T) {
	m := sized()
	m.consume(result{err: context.Canceled})

	for _, line := range spoken(m) {
		if strings.Contains(line, "canceled") || strings.Contains(line, "cancel") {
			t.Errorf("cancelled run printed a failure line: %q", line)
		}
	}
}