package tui

import (
	"context"
	"fmt"
	"strings"
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

// compacted holds one compaction pass's numbers, so the report and the test
// that checks it share one shape rather than a bag of named arguments.
type compacted struct {
	// evictCut is how many of the oldest messages this pass put aside.
	evictCut int

	// turns is how many of those the summary replaced. Zero when the
	// summarizer produced nothing and only the mask ran.
	turns int

	// kept is how many messages were left verbatim.
	kept int

	// results and thinking are how many of each the mask pass replaced.
	results  int
	thinking int
}

// compactOutcome is what a compaction pass sends back to the update loop. On
// the wire it carries the size before and the summary the backend produced
// (empty when there was no backend, or only the mask fallback is called for),
// plus any error from the summarizer call. The report/install functions fill
// in the after/size, the compacted tallies and the rebuilt conversation from
// those, so the UI thread installs one value instead of racing a goroutine
// that rewrites a list it is also reading.
type compactOutcome struct {
	conversation []nacelle.Message
	before       int64
	after        int64
	done         compacted
	summary      string
	err          error
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

	// compactMinResult is the smallest tool result worth masking. Under it,
	// the placeholder costs nearly what the result did, and a result that
	// small is usually load-bearing: an id, a path, a diff header.
	compactMinResult = 1024

	// compactSlack is how far under compactAt one pass aims to land, so a
	// session that grows between passes does not trim on every turn.
	compactSlack = 20_000

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
func runCompaction(m *Model, ctx context.Context, results chan compactOutcome, evictCut int) {
	defer close(results)
	conv := m.conversation
	outcome := compactOutcome{before: m.size}

	if agent := m.summarizer(); agent != nil {
		asks := compactPrompt(conv, evictCut)
		var b strings.Builder
		for event, err := range agent.Stream(ctx, asks) {
			if err != nil {
				outcome.err = err
				results <- outcome
				return
			}
			if event.Kind == nacelle.KindText {
				b.WriteString(event.Text)
			}
		}
		outcome.summary = strings.TrimSpace(b.String())
	}

	results <- outcome
}

// compactApply is the deterministic success half of a pass, split out of
// settleCompaction so a test can drive it: given the conversation, the
// eviction point, the summary that came back, and the size before, it returns
// the rebuilt conversation and how the pass spent itself. The evicted middle
// is replaced by the summary; the kept tail survives verbatim.
func compactApply(conv []nacelle.Message, evictCut int, summary string, before int64) compactOutcome {
	after := before - estTokens(convBytes(conv, evictCut)) + estTokens(len(summary))
	if after < 0 {
		after = 0
	}
	return compactOutcome{
		before:  before,
		after:   after,
		summary: summary,
		done: compacted{
			evictCut: evictCut,
			turns:    evictCut,
			kept:     len(conv) - evictCut,
		},
		conversation: compactedConversation(conv, evictCut, summary),
	}
}

// compactedConversation is the conversation a pass installs in place of the
// evicted middle: the summary as a user turn, then the kept tail, with the
// roles still alternating. Inserting the summary as a bare user message can
// land it directly before the kept tail's own user turn (a tool-result reply
// is a user message, and it is usually the oldest kept message), which the
// backends refuse as two consecutive user messages. When that would happen the
// summary is folded in as a leading text block of that same user turn instead,
// which keeps the canonical tool-call shape: the tool_result still follows its
// assistant tool_use, preceded by plain text that is equally legal there.
func compactedConversation(conv []nacelle.Message, evictCut int, summary string) []nacelle.Message {
	tail := conv[evictCut:]
	if len(tail) == 0 || tail[0].Role != nacelle.RoleUser {
		return append([]nacelle.Message{compactedHistory(summary)}, tail...)
	}
	front := tail[0]
	return append(
		[]nacelle.Message{{Role: nacelle.RoleUser, Parts: append([]nacelle.Part{nacelle.Text{Text: compactedHeader + summary}}, front.Parts...)}},
		tail[1:]...,
	)
}

// convBytes is the total byte weight of the first n messages, a stand-in for
// "what is about to be discarded" when estimating how much a pass frees.
func convBytes(conv []nacelle.Message, n int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += msgBytes(conv[i])
	}
	return total
}

// msgBytes is the byte weight of one message: the sum across its parts, which
// hold the tool output, the thinking and the spoken text, and nothing else
// that carries size worth counting.
func msgBytes(msg nacelle.Message) int {
	total := 0
	for _, part := range msg.Parts {
		total += partBytes(part)
	}
	return total
}

// partBytes is the byte weight of a single part: tool results and reasoning
// dominate a long session, and text is the message itself. Anything else
// weights nothing.
func partBytes(part nacelle.Part) int {
	if text, ok := part.(nacelle.Text); ok {
		return len(text.Text)
	}
	if result, ok := part.(nacelle.ToolResult); ok {
		return len(result.Result)
	}
	if reasoning, ok := part.(nacelle.Reasoning); ok {
		return len(reasoning.Text)
	}
	return 0
}

// maskEvicted is the cheap fallback stage: replace oversized tool results and
// thinking blocks in the evicted middle with placeholders, keeping the
// call/result pairing intact. It is synchronous, needs no backend, and runs on
// the UI thread, so it is the whole of compaction when no model is available
// to summarize. It mutates the conversation in place, which is safe here
// because it runs on the same thread that owns the conversation.
func (m *Model) maskEvicted(evictCut int) (int, int, int64) {
	budget := (m.size - m.compactAt + compactSlack) * 4
	if budget < 0 {
		budget = 0
	}
	results, thinking := 0, 0
	var saved int64
	for i := 0; i < evictCut && i < len(m.conversation); i++ {
		if budget > 0 {
			s, cut := m.trimResults(&m.conversation[i], budget)
			budget -= s
			saved += s
			results += cut
		}
		_, cut := m.trimThinking(&m.conversation[i], 0)
		thinking += cut
	}
	if saved > 0 {
		tokensSaved := saved / 4
		if tokensSaved > m.size {
			m.size = 0
		} else {
			m.size -= tokensSaved
		}
	}
	return results, thinking, saved
}

// applyMaskFallback runs the mask in place on the UI thread and reports it,
// for the two cases where the summary did not happen: the summarizer errored,
// or produced nothing. The mask still frees the bulky tool output, so a failed
// summary costs nothing but the attempt.
func (m *Model) applyMaskFallback(outcome compactOutcome) {
	evictCut := len(m.conversation) - keepCount(len(m.conversation))
	results, thinking, _ := m.maskEvicted(evictCut)
	done := compacted{
		evictCut: evictCut,
		kept:     len(m.conversation) - evictCut,
		results:  results,
		thinking: thinking,
	}
	report := compactOutcome{
		before:       outcome.before,
		after:        m.size,
		done:         done,
		conversation: m.conversation,
	}
	m.say(fromCompact, compactReport(report))
}

// trimResults replaces the droppable tool results of one message, up to
// budget bytes of original text, and reports how much it saved and how many
// results it replaced. Only user messages carry tool results, and the guard at
// the top makes this a quick no-op for assistant messages that trimThinking
// handles instead.
func (m *Model) trimResults(message *nacelle.Message, budget int64) (int64, int) {
	if message.Role != nacelle.RoleUser {
		return 0, 0
	}
	saved := int64(0)
	cut := 0
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
		cut++
	}
	return saved, cut
}

// trimThinking replaces long-form reasoning in old assistant messages with a
// placeholder, up to budget bytes of original text, and reports how much it
// saved and how many blocks it replaced. Reasoning that was dropped in an
// earlier pass is left alone.
//
// Thinking blocks are never sent back to the provider — nacelle drops them when
// building the request — so replacing them is invisible on the wire. What the
// model actually said (the Text part) stays untouched.
func (m *Model) trimThinking(message *nacelle.Message, budget int64) (int64, int) {
	if message.Role != nacelle.RoleAssistant {
		return 0, 0
	}
	saved := int64(0)
	cut := 0
	for j, part := range message.Parts {
		if budget > 0 && saved >= budget {
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
		cut++
	}
	return saved, cut
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

// compactedHistory is the message a pass installs in place of the evicted
// middle: one user turn holding a structured summary, labelled clearly so the
// model reads it as a compressed past rather than as anything new.
func compactedHistory(summary string) nacelle.Message {
	return nacelle.Message{
		Role:  nacelle.RoleUser,
		Parts: []nacelle.Part{nacelle.Text{Text: compactedHeader + summary}},
	}
}

// estTokens is the bytes-to-tokens estimate the whole file uses: four bytes
// per token is the rough English rate, and it is only ever compared against
// itself, so its error bars are directionally consistent.
func estTokens(bytes int) int64 {
	return int64(bytes) / 4
}

// settleCompaction installs a finished pass and starts the run that was
// waiting on the freed context. A summary that came back replaces the evicted
// middle; no summary (no backend, the call failed, or it came back empty)
// falls back to the mask on the UI thread. Either way the pass reports what
// it did, and it never leaves the conversation larger than it started.
func (m *Model) settleCompaction(outcome compactOutcome) tea.Cmd {
	m.compacting = false
	m.run.compactChan = nil

	evictCut := len(m.conversation) - keepCount(len(m.conversation))

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

// compactReport is how the client says a pass went: the context size it
// carried before and after, the share kept verbatim, what the mask and the
// summary did, and how much room came back. One line, so it fits the terminal
// and scrolls past without wrapping.
func compactReport(outcome compactOutcome) string {
	d := outcome.done
	freed := outcome.before - outcome.after
	kept := 0
	if d.evictCut+d.kept > 0 {
		kept = d.kept * 100 / (d.evictCut + d.kept)
	}

	var work []string
	if d.results > 0 {
		work = append(work, "masked "+countedNoun(d.results, "result"))
	}
	if d.thinking > 0 {
		work = append(work, "masked "+countedNoun(d.thinking, "thinking block"))
	}
	if d.turns > 0 {
		work = append(work, "summarized "+countedNoun(d.turns, "turn"))
	}
	if len(work) == 0 {
		work = []string{"kept everything verbatim"}
	}
	return fmt.Sprintf("✂ compacted context · %s → %s tokens · kept %d%% verbatim · %s · freed %s",
		shortTokens(outcome.before), shortTokens(outcome.after), kept, strings.Join(work, ", "), shortTokens(freed))
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

// compactSystem is the schema the summarizer writes against. Structured on
// purpose — the research on long-horizon agents is blunt that freeform
// summaries drop the load-bearing details — decisions, constraints, dead ends
// and exact state — that stop a model re-treading them. Verbatim identifiers
// survive so a model can still grep for the file or id a compressed summary
// names. The scoping lines are load-bearing too: the evicted middle is handed
// over alone, and the newer turns follow the summary unchanged, so the prompt
// must stop the model reaching past its chunk.
const compactSystem = "You are the compaction engine for a long-running coding agent. Your job " +
	"is to compress the older turns you are shown into a dense block that preserves everything " +
	"the working model still needs, so it can keep going as if those turns had happened — without " +
	"re-deriving them and without re-doing work. " +
	"Compress, do not reduce to a slogan. This is a compressed handoff of working memory, not a " +
	"prose recap, so keep the sharp edges that cause re-work: " +
	"the actual decisions made and the reasons, not just the conclusion; " +
	"constraints that must still hold — invariants, formats, interface contracts, security rules — " +
	"verbatim when short; " +
	"the state of the work — what exists, what is in flight, what was verified vs assumed; " +
	"dead ends and failed approaches, so the model does not re-try them; and " +
	"load-bearing identifiers verbatim — file paths, package and module names, function, class and " +
	"variable names, command invocations, tool and call ids, message ids, exact error strings, " +
	"version pins, and the config keys and values the work depends on. " +
	"Name the artifacts the work produced or touched, with their paths. " +
	"Structure the summary as short bullet sections, in exactly this order and only these: " +
	"Decisions, Constraints, Plan, State, Artifacts, Ruled out, Open questions. " +
	"Leave a section out if it is empty. Never add prose outside the bullets — no preamble, no " +
	"closing line. " +
	"Never invent facts that are not in the source: no guesses, no reconstructed numbers, no " +
	"unstated intentions. If something is genuinely ambiguous, record it under Open questions " +
	"instead of assuming. " +
	"Summarize only the turns shown to you. The newer turns after this chunk are preserved " +
	"verbatim elsewhere and will follow your summary unchanged, so do not anticipate, reference " +
	"or restate them — your summary must hand off the past without overlapping the present. " +
	"Be as short as correctness allows."

// compactAsk is the message tacked onto the evicted middle to ask for the
// summary. It repeats the chunk boundary because the model may lose the
// system's framing and try to "recap the whole conversation".
const compactAsk = "Above are the older turns to compact, and nothing else. Write the compaction " +
	"summary of exactly those turns, following your instructions. The conversation after this " +
	"chunk is kept intact and is not part of this request. Return only the summary block — no " +
	"preamble, no closing remark."

// compactPrompt is what the summarizer is fed: exactly the evicted middle with
// the ask appended. The kept tail is deliberately excluded, so the model
// cannot leak the present into the past's summary. It is a function so the
// chunk boundary is testable rather than buried in the goroutine.
func compactPrompt(conv []nacelle.Message, evictCut int) []nacelle.Message {
	return append(conv[:evictCut], nacelle.UserText(compactAsk))
}

// compactedHeader opens the message installed in place of the summarized
// middle, so the model reads it as a compressed past rather than as a new turn.
const compactedHeader = "[compacted context — the earlier turns were summarized]:\n\n"

// droppedNotice opens the text a masked result is replaced with. The mask
// skips any result already opening with it, so a second pass never pays a
// placeholder twice or counts it as savings.
const droppedNotice = "[dropped "

// droppedThinkingNotice opens the text a masked thinking block is replaced
// with. It plays the same role as droppedNotice, and uses the same "dropped"
// prefix so the reader sees it as the same mechanism.
const droppedThinkingNotice = "[dropped thinking: "
