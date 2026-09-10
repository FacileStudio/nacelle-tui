package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/nacelle-tui/internal/sessions"
	"github.com/FacileStudio/nacelle-tui/internal/usage"
)

// account holds what the client knows about what the session has spent: the
// usage total, and what it knows about how much context the conversation is
// carrying.
//
// The two live together because they answer one question from two sides —
// what the session cost, and whether the conversation is now too heavy to
// continue as-is — and both outlive any single run.
type account struct {
	// spent is the session's cumulative usage; the status line adds the run
	// in flight to it into a total that only ever goes up.
	spent nacelle.Usage

	// size is the input cost of the most recent finished turn, in tokens.
	// Every turn re-bills the whole conversation as input, so this is also
	// what the conversation would cost to send again right now — measured,
	// by the backend's own accounting, not guessed from bytes.
	size int64

	// trimmed is how many tool results and thinking blocks have been masked.
	// It reaches the status line, because a model whose memory was quietly
	// edited should not be the only one who knows.
	trimmed int

	// compactBegan is when the current compaction pass started, stamped in
	// beginCompaction. The running-tool row draws its elapsed time against
	// it, the same way every other live row draws against its group's start.
	compactBegan time.Time

	// began is when this client started, stamped once in newModel. The
	// status line and the closing recap both measure against it, so the
	// two can never disagree about how long the session ran.
	began time.Time

	// tools and failed are how many tool calls this session finished, and
	// how many of those fell over. They are counted where a call ends
	// rather than derived at the end from anything, because nothing keeps
	// a record of a finished call: the line is printed and forgotten.
	tools  int
	failed int

	// sink appends each finished turn to mycelium's event feed, so a session
	// running here shows up in mycelium's dashboard while it is still going.
	// It is nil when mycelium is not installed on this machine.
	sink *usage.Sink

	// session is this run of the client written down: the questions asked
	// and the answers given, appended to a file under ~/.nacelle/sessions
	// as they are said. It is nil when the file could not be opened, which
	// is not a reason to refuse to run.
	session *sessions.SessionLog

	// tasks is the plan the model is working to, as it last reported it.
	// It is written only by the routed update, never by the tool that
	// produces it — see tasks.go for why a tool goroutine cannot touch it.
	tasks taskList
}

// compactFinished says the compaction channel closed and no outcome arrived.
type compactFinished struct{}

const (
	// compactKeepMessages is how many of the newest messages are never
	// touched. What the model is reasoning about right now lives here;
	// dropping it would save tokens and lose the plot.
	compactKeepMessages = 4

	// compactKeepPercent is how much of a long conversation stays verbatim.
	// Compaction is a hybrid: the newest this fraction of turns is kept
	// intact and the older middle is summarized, so the model keeps its feet
	// in the recent thread while the distant past is compressed.
	compactKeepPercent = 25

	// compactMaxTokens is the ceiling a compaction summary is asked to stay
	// under. Small on purpose: the summary replaces a large middle with a
	// stub, so a long summary would buy almost nothing.
	compactMaxTokens = 2000
)

// sized records what a finished turn cost on the input side.
//
// Cache reads and cache creations are billed input like any other, and both
// backends report them here, so leaving either out would understate the
// conversation by most of an agentic session's real size.
func (m *Model) sized(usage nacelle.Usage) {
	m.size = usage.InputTokens + usage.CacheReadTokens + usage.CacheCreationTokens
}

// keepCount is how many of the newest messages survive a pass verbatim: at
// least compactKeepMessages, and a share of a long conversation beyond that.
// A conversation no longer than the floor is kept whole, so a short session is
// never compacted. evictCut, the number of oldest messages a pass may touch,
// is len minus it.
func keepCount(length int) int {
	if length <= compactKeepMessages {
		return length
	}
	n := length * compactKeepPercent / 100
	if n < compactKeepMessages {
		n = compactKeepMessages
	}
	if n > length-1 {
		n = length - 1
	}
	return n
}

// beginCompaction runs one pass. It is called from send, right before the
// model needs the freed context, and only when the conversation is over the
// threshold. It is where the running-tool row and the purple status come
// from: a pass is an LLM call now, so it must not block the update loop.
//
// The pass is two-stage and tiered, the way the research on long-running
// agents lands. The summarization stage runs first, on its own goroutine,
// against the raw evicted middle — it must read the actual tool output and
// decisions, not the mask's placeholders, so the summary can preserve exactly
// what compaction would otherwise discard. The mask stage is the fallback: if
// there is no backend, or the summary call fails, or comes back empty, the UI
// thread masks the oversized tool results and thinking blocks in the evicted
// middle as a cheap headroom lever. Either way a pass never hands a model more
// context than it started with, and only the newest share of messages is kept
// verbatim in both stages.
//
// Nothing is recorded to the session file — this is the client talking about
// itself, the same reason the banner is not.
func (m *Model) beginCompaction(ctx context.Context) tea.Cmd {
	evictCut := len(m.conversation) - keepCount(len(m.conversation))
	if evictCut <= 0 || m.compacting {
		return nil
	}

	m.compacting = true
	m.compactBegan = time.Now()

	resultsChan := make(chan compactOutcome)
	m.run.compactChan = resultsChan

	go runCompaction(m, ctx, resultsChan, evictCut)

	return waitForCompact(resultsChan)
}

// runCompaction is the pass's own goroutine. It asks the backend for a
// summary of the raw evicted middle and sends back just that result; it never
// mutates the conversation. Because the conversation is stable while a pass is
// in flight — send holds the run busy — reading it here is safe.
//
// The summarizer runs inside a deadline set by summarizeInto, so a wedged
// backend cannot hold the session at "compacting" forever: whichever way the
// stream winds down once the deadline fires, the outcome still arrives and
// the pass falls back to the mask.
func runCompaction(m *Model, ctx context.Context, results chan compactOutcome, evictCut int) {
	defer close(results)
	conv := m.conversation
	outcome := compactOutcome{before: m.size, evictCut: evictCut}

	if agent := m.summarizer(); agent != nil {
		summary, err := summarizeInto(ctx, agent, compactPrompt(conv, evictCut))
		if err != nil {
			outcome.err = err
		} else {
			outcome.summary = summary
		}
	}

	results <- outcome
}

// summarizer builds the small, tool-free agent asked to compact the evicted
// middle. It runs on the same backend as the session, so the summary is
// billed exactly like the work it protects, and it is nil when there is no
// backend — tests and offline runs mask instead of summarizing.
func (m *Model) summarizer() *nacelle.Agent {
	if m.agent == nil {
		return nil
	}
	agent, err := nacelle.New(nacelle.Config{
		Backend:       m.agent.Backend(),
		System:        compactSystem,
		Thinking:      nacelle.Thinking{},
		MaxTokens:     compactMaxTokens,
		MaxIterations: 1,
	})
	if err != nil {
		return nil
	}
	return agent
}

// settleCompaction installs a finished pass and starts the run that was
// waiting on the freed context. A summary that came back replaces the evicted
// middle; no summary (no backend, the call failed, or it came back empty)
// falls back to the mask on the UI thread. Either way the pass reports what
// it did, and it never leaves the conversation larger than it started.
func (m *Model) settleCompaction(outcome compactOutcome) tea.Cmd {
	m.compacting = false
	m.run.compactChan = nil

	evictCut := outcome.evictCut

	if outcome.err != nil {
		m.applyMaskFallback(outcome)
		m.say(fromCompact, "compaction summary failed · "+outcome.err.Error()+" — masked instead")
	} else if outcome.summary != "" {
		done := compactApply(m.conversation, evictCut, outcome.summary, outcome.before)
		m.conversation = done.conversation
		m.size = done.after
		m.say(fromCompact, compactReport(done))
	} else {
		m.applyMaskFallback(outcome)
	}

	if m.run.busy && m.agent != nil {
		return m.startRun(m.run.bgCtx)
	}
	return nil
}

// waitForCompact takes exactly one outcome and re-arms itself from Update,
// the same contract waitFor holds for run results.
func waitForCompact(results <-chan compactOutcome) tea.Cmd {
	return func() tea.Msg {
		next, open := <-results
		if !open {
			return compactFinished{}
		}
		return next
	}
}
