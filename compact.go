package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/FacileStudio/nacelle"
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

	// trimmed is how many tool results and thinking blocks have been replaced.
	// It reaches the status line, because a model whose memory was quietly
	// edited should not be the only one who knows.
	trimmed int

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
	sink *usageSink

	// session is this run of the client written down: the questions asked
	// and the answers given, appended to a file under ~/.nacelle/sessions
	// as they are said. It is nil when the file could not be opened, which
	// is not a reason to refuse to run.
	session *sessionLog

	// tasks is the plan the model is working to, as it last reported it.
	// It is written only by the routed update, never by the tool that
	// produces it — see tasks.go for why a tool goroutine cannot touch it.
	tasks taskList
}

const (
	// compactAt is the input-token size past which old tool results start
	// being dropped. It sits well inside the smallest window nacelle is
	// aimed at, so the first sign of trouble is never StopContext: the
	// floor it leaves below itself is room for a full answer plus the next
	// turn's tools and system prompt.
	compactAt = 100_000

	// compactKeepMessages is how many of the newest messages are never
	// touched. What the model is reasoning about right now lives here;
	// dropping it would save tokens and lose the plot.
	compactKeepMessages = 4

	// compactMinResult is the smallest tool result worth replacing. Under
	// it, the placeholder costs nearly what the result did, and a result
	// that small is usually load-bearing: an id, a path, a diff header.
	compactMinResult = 1024

	// compactSlack is how far under compactAt one pass aims to land, so a
	// session that grows between passes does not trim on every turn.
	compactSlack = 20_000
)

// sized records what a finished turn cost on the input side.
//
// Cache reads and cache creations are billed input like any other, and both
// backends report them here, so leaving either out would understate the
// conversation by most of an agentic session's real size.
func (m *model) sized(usage nacelle.Usage) {
	m.size = usage.InputTokens + usage.CacheReadTokens + usage.CacheCreationTokens
}

// compact drops old tool results and thinking blocks once the conversation
// outgrows compactAt.
//
// Tool results and thinking blocks are what get dropped, deliberately. In an
// agentic session they are the bulk of the context — a file read, a grep over
// a tree, a test run, a chain-of-thought — and their value decays fastest: by
// ten turns later the model needs the fact it found, not the forty kilobytes it
// arrived inside, nor the reasoning that led to it. Assistant text stays,
// because it is the conclusion, and everything recent stays, because that is
// what the next turn is about.
//
// A replaced result keeps its ToolResult shell — same id, same name, same
// failure flag — with the body swapped for a note saying what happened and
// how to get it back. That keeps the call/result pairing every provider
// validates, tells the model plainly that information was removed rather
// than never existing, and names the way back: run the tool again.
//
// One pass trims toward compactAt minus compactSlack and stops there, so a
// session sitting just over the line does not shave a result off every turn.
//
// The threshold is high on purpose beyond leaving room to answer: rewriting
// any message invalidates the provider's prompt cache for everything after
// it, so the turn after a pass re-bills the whole prefix as a cache write.
// Compacting rarely, and deeply when it does, amortizes that bust; trimming
// a little every turn would pay it every turn and save nothing.
//
// The trim budget is in bytes rather than tokens: size counts tokens, four
// bytes per token is the rough English rate, and comparing the two directly
// would trim about a quarter of what was intended.
func (m *model) compact() {
	if m.size <= compactAt || len(m.conversation) <= compactKeepMessages {
		return
	}
	budget := (m.size - compactAt + compactSlack) * 4
	limit := len(m.conversation) - compactKeepMessages
	for i := 0; i < limit && budget > 0; i++ {
		budget -= m.trimResults(&m.conversation[i], budget)
		budget -= m.trimThinking(&m.conversation[i], budget)
	}
}

// trimResults replaces the droppable tool results of one message, up to
// budget bytes of original text, and reports how much it saved. Only user
// messages carry tool results, and the guard at the top makes this a quick
// no-op for assistant messages that trimThinking handles instead.
func (m *model) trimResults(message *nacelle.Message, budget int64) int64 {
	if message.Role != nacelle.RoleUser {
		return 0
	}
	saved := int64(0)
	for j, part := range message.Parts {
		if saved >= budget {
			break
		}
		result, ok := part.(nacelle.ToolResult)
		if !ok || len(result.Result) < compactMinResult || strings.HasPrefix(result.Result, droppedNotice) {
			continue
		}
		saved += int64(len(result.Result))
		message.Parts[j] = nacelle.ToolResult{
			ID:     result.ID,
			Name:   result.Name,
			Failed: result.Failed,
			Result: fmt.Sprintf("%s%d bytes%s", droppedNotice, len(result.Result), ". Re-run the tool if the detail matters."),
		}
		m.trimmed++
	}
	return saved
}

// trimThinking replaces long-form reasoning in old assistant messages with a
// placeholder, up to budget bytes of original text, and reports how much it
// saved. Reasoning that was dropped in an earlier pass is left alone.
//
// Thinking blocks are never sent back to the provider — nacelle drops them when
// building the request — so replacing them is invisible on the wire. What the
// model actually said (the Text part) stays untouched.
func (m *model) trimThinking(message *nacelle.Message, budget int64) int64 {
	if message.Role != nacelle.RoleAssistant {
		return 0
	}
	saved := int64(0)
	for j, part := range message.Parts {
		if saved >= budget {
			break
		}
		reasoning, ok := part.(nacelle.Reasoning)
		if !ok || reasoning.Text == "" || strings.HasPrefix(reasoning.Text, droppedThinkingNotice) {
			continue
		}
		saved += int64(len(reasoning.Text))
		message.Parts[j] = nacelle.Reasoning{
			Text: fmt.Sprintf("%s%d bytes%s", droppedThinkingNotice, len(reasoning.Text), ". See the assistant text above for the conclusion."),
		}
		m.trimmed++
	}
	return saved
}

// droppedNotice opens the text a trimmed result is replaced with. Compact
// skips any result already opening with it, so a second pass never pays a
// placeholder twice or counts it as savings.
const droppedNotice = "[dropped "

// droppedThinkingNotice opens the text a trimmed thinking block is replaced
// with. It plays the same role as droppedNotice, and uses the same "dropped"
// prefix so the reader sees it as the same mechanism.
const droppedThinkingNotice = "[dropped thinking: "
