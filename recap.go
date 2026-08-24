package main

import (
	"fmt"
	"time"
)

// recap is the last thing a reader sees: what the session ran and what it
// cost, in two lines, or nothing at all for a session that did none of it.
//
// It exists because the status line is not a record. Everything this client
// draws lives in the frame View owns, and the frame is erased when the program
// gives the terminal back — so the one number somebody wanted to keep, what
// the session cost, disappears at exactly the moment they went looking for it.
// Printing it after Run returns puts it in the shell's own scrollback instead,
// below the session and above the next prompt, where it survives the client
// entirely. See View's doc comment for why that works here and would not in an
// alternate screen: there is no page to un-draw, so ordinary printing simply
// appends to a session that is already there.
//
// A session that ran no tool and spent no token gets no recap, and the
// threshold is exactly that rather than a duration. Someone who launched this
// by mistake and quit is the obvious case, but so is someone who left it open
// for an hour without asking anything — time sitting at a prompt is not work,
// and a table of zeroes for it reads as a client that failed rather than one
// that was never used. Anything that reached the backend, however briefly, is
// worth a line; nothing that did is not.
//
// The total is what was spent plus the run in flight, the same sum the status
// line was showing a moment earlier. Quitting mid-answer is a normal way to
// leave, and a recap that dropped that run's tokens would report a smaller
// figure than the line it just replaced, for a run the reader is still billed
// for.
//
// A zero is left out rather than printed. "0 failed" is the absence of news
// given the same width as news, and it is the tool count itself that carries
// whether anything ran. Cost follows the call status.go already made and is
// shown only when a backend reported one — Anthropic returns tokens and
// nothing else, and $0.0000 reads as free rather than as unknown.
//
// Nothing here is styled. The program has handed the terminal back by the time
// this is printed, so colour would be this client painting a shell it no
// longer owns, in a palette it resolved for a frame that is gone.
func (m *model) recap() string {
	total := m.spent.Add(m.run.usage)
	tokens := total.InputTokens + total.OutputTokens + total.CacheReadTokens + total.CacheCreationTokens
	if m.tools == 0 && tokens == 0 {
		return ""
	}

	shape := "session · " + lasted(time.Since(m.began))
	switch {
	case m.tools == 1:
		shape += " · 1 tool"
	case m.tools > 1:
		shape += fmt.Sprintf(" · %d tools", m.tools)
	}
	if m.failed > 0 {
		shape += fmt.Sprintf(" · %d failed", m.failed)
	}

	spend := fmt.Sprintf("in %s · out %s",
		shortTokens(total.InputTokens+total.CacheCreationTokens),
		shortTokens(total.OutputTokens))
	if total.CacheReadTokens > 0 {
		spend += fmt.Sprintf(" · %s cached", shortTokens(total.CacheReadTokens))
	}
	if total.Cost > 0 {
		spend += fmt.Sprintf(" · $%.4f", total.Cost)
	}
	return shape + "\n" + spend
}

// lasted is how long the session ran, at the resolution a closing line wants.
//
// Whole seconds, because Go's own rendering of a duration nobody rounded is
// `14m3.0021847s`, and the digits after the decimal point are the only part of
// a session length nobody has ever cared about. It floors at one second for
// the same reason took does: `session · 0s` over a session that plainly did
// something reads as a broken clock.
func lasted(spent time.Duration) string {
	return max(spent.Round(time.Second), time.Second).String()
}
