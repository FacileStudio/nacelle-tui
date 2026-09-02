# Plan: Close the remaining compaction gaps in nacelle-tui

Checked against: nacelle SDK conventions, module path (`github.com/FacileStudio/nacelle-tui`), filet gate (already clean).

---

## Goal

Three independent improvements to the conversation compaction system in nacelle-tui (and the nacelle SDK where needed), so large sessions stay under the context window without losing the plot or paying for thinking blocks they no longer need.

## Why

The current `compact()` runs *after* a turn, using that turn's billed usage — so a session can sail past `compactAt` undetected between turns. Thinking blocks (`nacelle.Reasoning`) are never trimmed, and Anthropic's server-side compaction beta is unwired entirely. Observed: compact.go line 122 calls `compact()` from settle.go line 49, after `m.size` is set by `m.run.usage`; `send()` at run.go:242 launches the stream with no pre-flight check.

## Approach

Three parallel tracks, independent except for one cross-repo dependency in track 3. Each is the smallest viable change.

---

## Track 1 — Pre-flight CountTokens (nacelle-tui only)

### Why

A question that adds 50KB of tool results to a conversation already at 95K tokens would start a turn past compactAt, and compact() only runs after that turn finishes. The backend already paid the full price by then.

### What

In `send()` (run.go:242), before `start()`, call `m.agent.CountTokens(ctx, m.conversation)`. If the count exceeds `compactAt`, run `compact()` synchronously — it already works on `m.size` and `m.conversation` — then call `CountTokens` once more to verify. If still above the window, clamp `compactAt` lower or warn the user. Then send the conversation.

### Files to modify

- `run.go` — add pre-flight check at line ~255, before `m.run.results = start(...)`
- `run.go` — add a `countTokens` (or call `agent.CountTokens` inline, its interface is `CountTokens(ctx, conversation)` → `(int64, error)`)
- `compact.go` — no change needed; `compact()` already works on the conversation and its `compactAt` constant

### Steps

1. `run.go` — add a `preflight()` method or inline check in `send()`:
   - derive a context from `context.Background()` (or use the run's own ctx, which isn't created yet — so a fresh `context.WithTimeout(context.Background(), 5*time.Second)`)
   - call `m.agent.CountTokens(ctx, m.conversation)`
   - if count > compactAt + compactSlack: `m.size = count` then `m.compact()` then re-check
   - only then launch `start()`
2. Add a test in `compact_test.go` or new `preflight_test.go`:
   - stub agent with known CountTokens return
   - verify that send() compacts before launching when count > compactAt
   - verify that a conversation just under compactAt launches as-is

### Exit criteria

- `filet check` passes
- Tests pass: `go test ./...`
- A conversation at 110K tokens compacts before sending instead of after

### Risk

- `CountTokens` is a new API call per question, adding latency. Budget 5s timeout; on failure (network blip, OpenRouter doesn't support it), log and send anyway — the post-turn compact still works.
- Some backends don't support `CountTokens` (OpenRouter returns `*Unsupported`). Catch that error and skip pre-flight.

---

## Track 2 — Trim old thinking blocks (nacelle-tui only)

### Why

`compact()` skips assistant messages entirely (line 138: `if message.Role != nacelle.RoleUser { return 0 }`). In a long reasoning session, the thinking blocks (`nacelle.Reasoning` parts) on old assistant turns are the bulk of the context, and they decay as fast as tool results do — by ten turns later the model needs the conclusion, not the chain-of-thought it took to reach it.

### What

Extend `trimResults()` (or add `trimThinking()`) to replace `Reasoning` parts on non-recent assistant messages with a placeholder, in the same compact pass. The placeholder says which old reasoning was removed and that the conclusion (the `Text` part) is still there.

### Files to modify

- `compact.go`:
  - `trimResults()` already handles `nacelle.ToolResult` — can either expand its type switch or add a sibling `trimThinking()` called from the same loop
- `compact_test.go`:
  - Add a conversation with `nacelle.Reasoning` parts, verify they get dropped past the keep window

### Steps

1. `compact.go` — in the `compact()` loop (line 122-131), add a second pass or extend the existing one to trim thinking:
   - For assistant messages outside the keep window, scan parts for `nacelle.Reasoning`
   - Replace each with a placeholder: `nacelle.Reasoning{Text: fmt.Sprintf("[thinking: %d bytes — see the assistant text above for the conclusion]", len(original))}`
   - Update `m.trimmed` counter (or add a `m.thoughtsTrimmed` if you want separate stats)
   - Use the same budget: thinking blocks count against the byte budget
2. `compact_test.go` — add `TestCompactDropsOldThinkingBlocks`:
   - Build a conversation with a `nacelle.Reasoning` part on an early assistant message
   - Verify it's replaced outside the keep window and untouched inside it

### Exit criteria

- All compaction tests pass
- Thinking blocks in recent messages are untouched
- Old thinking blocks are replaced, not removed (pairing shape preserved — same number of parts)

### Risk

- Zero: `Reasoning` is never sent back to the backend (see nacelle/message.go lines 55-66), so replacing it is invisible to the wire format. The text part of the same assistant message survives.
- The budget arithmetic in `compact()` uses bytes (4× token count). Thinking blocks tend to be dense prose, but the 4× heuristic still holds roughly.

---

## Track 3 — Wire Anthropic server-side compaction (nacelle SDK + nacelle-tui)

### Why

Anthropic offers a `context_management` server-side compaction beta that summarises old messages in the request, saving context without the client having to guess what to drop and without invalidating the prompt cache on every edit. Currently `stop_reason: compaction` is already mapped to `nacelle.StopOther` in the Anthropic backend (anthropic/stop_test.go line 44), but the beta header is never attached and the TUI treats StopOther as an error-ish ending.

### What

Two changes:

1. **nacelle SDK (anthropic backend):** Add the `compact_20260112` beta header (and the `context_management` config) to the request when the client opts in. When the server compacts mid-run, the `stop_reason: compaction` triggers `StopOther` as it already does — but the client can distinguish "compaction" from other unknowns and retry.

2. **nacelle-tui:** Detect when a run ends with `StopOther` due to compaction and retry the same question automatically (without showing an error). Add optional client-side opt-in config.

### Files to modify

**nacelle SDK:**
- `anthropic/anthropic.go` — add a `Compaction` option to `Config`, and if set, add `sdk.AnthropicBetaCompaction2026_01_12` (or the raw string constant `"compact_20260112"`) to `params.Betas`
- `anthropic/anthropic.go` — possibly also set `context_management` on the request (needs SDK field check)

**nacelle-tui:**
- `config.go` — add `compaction: true` YAML field on the `Reasoning` or `MCP` block (or a new top-level `compaction` field)
- `merge.go` — merge the compaction flag
- `settle.go` — in `settle()`, after closeTurn/compact, check if `m.run.stop == nacelle.StopOther` and the model supports compaction; if so, re-ask the same question instead of returning to prompt
- `flags.go` — add a `-compaction` flag

### Steps

1. `nacelle/anthropic/anthropic.go`:
   - Add `Compaction bool` to `Config`
   - In `params()`, if `Compaction` is true, append the beta to `params.Betas`
   - Verify the SDK constant name (anthropic-sdk-go may call it `BetaCompaction2026_01_12` or similar)
2. `nacelle/anthropic/anthropic_test.go`:
   - Add a test verifying the beta header is present when compaction is on
3. `nacelle-tui/config.go`:
   - Add `Compaction bool` to the config struct (likely on `Reasoning` or as a sibling of `ShowThinking`)
4. `nacelle-tui/merge.go`:
   - Merge the compaction flag from file/flag/env
5. `nacelle-tui/flags.go` or `config.go`:
   - Wire `compaction` flag & env var
6. `nacelle-tui/settle.go`:
   - After `m.compact()`, if `m.run.stop == nacelle.StopOther && config.Compaction`, re-ask the question that was just answered (the last user text in `m.conversation`) instead of waiting for input
7. `nacelle-tui/agent.go`:
   - Pass compaction flag through `build()` to the backend config
8. New or updated test in `settle_test.go` or new `compaction_test.go`:
   - Verify stop=StopOther with compaction on re-asks the question

### Exit criteria

- `filet check` and tests pass on both repos
- With `compaction: true`, a conversation that triggers server-side compaction retries without showing an error
- Without `compaction: true`, the beta header is absent and behaviour is unchanged (StopOther stays as-is, no retry)

### Risk

- **SDK dependency:** The anthropic-sdk-go constant name may differ from what's assumed here. Need to verify before implementation: `rg "BetaCompaction|Compaction"` in vendor or go.sum.
- **Cross-repo:** Both nacelle and nacelle-tui need to change. The SDK change is small and backward-compatible, so publish as nacelle v0.6.0 first, then nacelle-tui depends on it (go get commit or tag).
- **Server-side compaction is beta:** It could change behaviour or stop working. The opt-in flag means it's off by default — no risk for existing users.
- **Prompt cache:** Server-side compaction edits the conversation server-side, which invalidates the prompt cache for future turns — same as client-side compaction does. The trade-off is the same.

---

## Order

All three tracks are independent in design. Implementation order:

1. **Track 1 (pre-flight)** — pure nacelle-tui, no dep, highest value (prevents the worst case)
2. **Track 2 (thinking trimming)** — pure nacelle-tui, no dep, adds coverage to the existing `compact()` pass
3. **Track 3 (server-side compaction)** — cross-repo, beta feature, opt-in

Tracks 1 and 2 can run in parallel if two workers are available. Track 3 must be sequenced after both are merged (or at least after the compact() surface is stable).

---

## Files to Modify / New

**nacelle-tui:**
- `run.go` — pre-flight CountTokens
- `compact.go` — thinking block trimming
- `compact_test.go` — thinking block trimming tests + pre-flight tests
- `config.go` — compaction flag
- `merge.go` — compaction merge
- `flags.go` — compaction CLI flag
- `settle.go` — StopOther retry for compaction
- `agent.go` — pass compaction to backend config
- (new) `preflight_test.go` or tests in `compact_test.go`

**nacelle SDK:**
- `anthropic/anthropic.go` — Config.Compaction + beta header
- `anthropic/anthropic_test.go` — beta header test

---

## Skip (YAGNI)

- **Summarisation fallback** — a client-side summarisation step that calls the model to condense old messages is deliberately out of scope. Server-side compaction covers the same need with less complexity; if it's unavailable, the existing tool-result dropping + thinking block trimming already shrinks the conversation dramatically. Add summarisation only if those two prove insufficient.
- **Smart router that picks which backend's compaction to use based on capability** — the TUI already hard-codes the backend choice per run; the compaction feature flag is per-model-config. A router adds complexity that nobody has asked for.
- **Persistent compaction state** — the trimmed count is already in the status line. Storing it on disk between sessions is not needed.
- **Graceful degradation for OpenRouter** — OpenRouter doesn't support CountTokens or server-side compaction. The pre-flight check skips it with `*Unsupported`, and server compaction is opt-in. No degradation needed.

---

Pause for review: does this match what you had in mind? Any tracks you'd cut, reorder, or add detail to?