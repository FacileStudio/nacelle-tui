package tui

import (
	"fmt"
	"strings"

	"github.com/FacileStudio/nacelle"
)

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
	evictCut     int
	summary      string
	err          error
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

// compactedHistory is the message a pass installs in place of the evicted
// middle: one user turn holding a structured summary, labelled clearly so the
// model reads it as a compressed past rather than as anything new.
func compactedHistory(summary string) nacelle.Message {
	return nacelle.Message{
		Role:  nacelle.RoleUser,
		Parts: []nacelle.Part{nacelle.Text{Text: compactedHeader + summary}},
	}
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

// compactPrompt is what the summarizer is fed: exactly the evicted middle with
// the ask appended. The kept tail is deliberately excluded, so the model
// cannot leak the present into the past's summary. It is a function so the
// chunk boundary is testable rather than buried in the goroutine.
func compactPrompt(conv []nacelle.Message, evictCut int) []nacelle.Message {
	return append(conv[:evictCut], nacelle.UserText(compactAsk))
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

// compactedHeader opens the message installed in place of the summarized
// middle, so the model reads it as a compressed past rather than as a new turn.
const compactedHeader = "[compacted context — the earlier turns were summarized]:\n\n"
