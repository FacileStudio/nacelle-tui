# Changelog

All notable changes to `nacelle-tui` are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow semver —
while on `v0`, a breaking change bumps the minor.

## [Unreleased]

### Added

- **Headless mode** (`-print "prompt"`): builds the agent the same way the TUI
  does, streams text to stdout, no bubbletea. Piped stdin works when `-print` is
  passed alone (`echo "list files" | nacelle`). Track G.
- **Settings chain extracted to `internal/settings/`**: `Config`, `Toggles`,
  `UI`, `Reasoning`, `Web`, `Discovery`, `Sources`, `HookSpec` and the full
  precedence chain (defaults → file → env → flags) now live in their own
  package. Net -779 lines in `package main`. Track G.

## [0.16.1] — 2026-09-02

### Fixed

- **Trailing whitespace clipped** from streaming ansified text, and every
  completed line committed to scrollback rather than whole paragraphs.

## [0.16.0] — 2026-09-02

No changelog recorded — minor fixes between 0.15.0 and 0.16.1.

## [0.15.0] — 2026-09-02

### Added

- **Duplicate-key tool input is refused unconditionally** — `strictObject` now runs
  on every tool call, not only when `-approve-tools` is on. A call with a repeated
  JSON key (`{"command":"ls","command":"rm -rf /"}`) is refused as ambiguous before
  the tool runs, fixing the display/execution split where the transcript showed
  one value while the tool decoded another. Track D3.
- **Identical consecutive tool failures collapse into one line with a count** —
  when the same tool fails with the same error twice in a row, the transcript
  shows `run_command failed 2 times` instead of two identical two-line blocks.
  Track D2.

## [0.14.0] — 2026-09-02

### Added

- **Pre-flight token count compaction**: `send()` now calls `agent.CountTokens()`
  before launching the stream. If the conversation already exceeds `compactAt`,
  compaction runs synchronously before the turn starts — catching growth that
  happened between turns rather than only after a turn finishes. Silent skip
  on backends that don't support `CountTokens` (OpenRouter).
- **Old thinking blocks are now trimmed alongside tool results**: `compact()`
  replaces `nacelle.Reasoning` parts outside the keep window with a
  `[dropped thinking: N bytes...]` placeholder. Thinking is never sent back to
  the API, so the replacement is invisible on the wire; the assistant's
  conclusion (its `Text` part) stays untouched.
- **Checkmark icon and green styling on the "ready" status line**: when the agent
  finishes work and the client is idle, the status line now shows `✓ ready` in
  green (ANSI 2) instead of plain `ready`. A new `palette.ready` style in
  `theme.go` holds the green foreground; `status()` uses it only for the idle
  state, so busy, stopping, and approval prompts keep their own colours.

### Fixed

- **Tool glyph colour was ANSI SGR 1 (bold) and 2 (dim), not 31 (red) and 32 (green)**.
  The `colorGlyph` calls in `finished()` used `"1"` for failure and `"2"` for
  success, which emits `\x1b[1m` (bold) and `\x1b[2m` (dim) instead of the
  intended red and green foreground colours. A failed tool's icon showed bold
  but not red; a successful one showed dim but not green. Both now use the
  correct 3-digit ANSI colour codes, so the icon actually turns green on
  success and red on failure.

## [0.12.0] — 2026-09-02

### Added

- **Blocked and failed task statuses**: a step that hits trouble now marks
  itself `blocked` or `failed` with a `reason`, rather than lying or hanging
  forever. Glyphs: `⊗` (red) for failed, `⊘` (yellow) for blocked.
- **`step_update` mode**: update one step by its 0-based index instead of
  resending the whole plan. Lighter for status changes, fewer tokens per call.
- **Task reminder in settle**: when a run ends normally with unfinished steps,
  a line prints (`☆ tasks: 3/5 steps complete`) and a user-message is injected
  so the model sees it on the next turn.
- **System prompt rule**: "When you lay out work with the tasks tool, keep the
  plan current as you go" — visible to every model on every turn.
- **Markdown rendering for thinking traces**: expanded reasoning now renders
  through glamour like answers do, not just italic grey.

### Changed

- **Tool glyph colour restored after outcome marker**: the old code used
  `\x1b[m` (full reset) after the green/red check/cross, which killed the
  tool colour for the rest of the line. Now the glyph switches back to the
  tool's own hue so `✎ edit_file(view.go) · 12ms` reads as green marker +
  magenta tool line.
- **User question contrast**: `faintFg` moved to ANSI 15 (white) on dark
  terminals, so reader questions and queued messages are white text on the
  dark grey background instead of mid-grey on grey.

### Fixed

- **Thinking trace colour restored**: the third pass of tool-glyph colouring
  (commit ada4611) coloured only the glyph, which fixed tool lines but broke
  the agent's thinking traces — they lost their markdown rendering. Now both
  live-streaming and committed thinking use `m.markdown(text)` like answers do.

## [0.11.0] — 2026-09-02

### Added

- **Mid-line `/command` completion**: tab with more than one match opens the
  dropdown instead of silently picking the first. Re-selecting a command now
  preserves any text before and after the `/command` word.
- **Paragraph streaming**: complete paragraphs are committed to scrollback as
  they arrive, before the model finishes.
- **Token counter shows millions**: output like `2033k cached` now reads
  `2.0M cached`.

### Changed

- **`flush()` reads `fullAnswer`**: the entire conversation (not just the
  visible window) is printed when entering full scrollback.
- **Failed tool lines**: the tool call and its error are rendered as one entry
  instead of two, so there is no blank line between them.
- **Result `⤷ ` has no leading indent**: the two spaces before the arrow were
  removed.
- **Diff output keeps a trailing blank line** for better visual separation
  from the next transcript entry.

## [0.10.0] — 2026-09-01

### Fixed

- **Muted thinking trace** no longer printed to stdout.
- **Tool glyph colours** (green/red) apply only to the check/cross, not the
  whole line.
- **Queued/question backgrounds** are no longer muted.
- **Removed dead group functions** and formatted with gofmt.
- **Fixed embedded field selectors** for QF1008 compliance.

## [0.9.1] — 2026-09-01

### Fixed

- **Tool icon color only applies to the glyph, not the whole line**
  (regression in 0.9.0 where the entire tool line turned green/red).
- **Queued messages now have a background**, matching the prompt styling for
  better visual distinction from the transcript region.
- **Removed unused speaker types** (`fromToolOk`, `fromToolFail`) in favour of
  embedding ANSI color codes in the line string itself.

## [0.9.0] — 2026-09-01

### Fixed

## [0.8.0] — 2026-09-01

### Added

- **Mid-message `/command` suggestions**: typing `/` anywhere in the prompt
  now opens the dropdown menu, not only at the start. The word after the last
  slash is the filter, and selecting a command replaces only that word,
  leaving the rest of the sentence intact.
- **Prompt highlighting**: the prompt text turns cyan while a `/command` word
  is active, giving visual feedback that the menu is open.
- **Colored task icons**: `✓` (green) for completed, `▶` (cyan) for
  in_progress, `○` (muted) for pending. Characters are text-safe unicode, not
  emoji.
- **Scroll-aware dropdown**: when the match list exceeds the visible height,
  selection scrolls the window so the highlighted row stays on screen.
- **Tool grouping**: identical consecutive tool calls fold into a single
  transcript row labelled `(N×)`, reducing noise when the model reads the
  same file or searches the same pattern repeatedly. Toggle with
  `group_tools` in `~/.nacelle.yml`.

### Changed

- **Task list reordered** above the status line, so the loading/status row
  sits below the plan rather than above it.
- **Go toolchain** bumped to 1.26 across `mise.toml` and `go.mod`.

### Fixed

- **The `running` map is gone**, replaced by the group-based tracking. The old
  map was keyed by call ID and never cleaned up for discarded calls, and its
  iteration order was deliberately random, which made `stranded()` sort the
  keys first to keep the transcript stable. The group list is order-preserving
  by construction, and a discarded group is simply not added.

## [0.7.0] — 2026-09-01

### Added

- **`google` and `openai` backends**: `-backend google` runs on Google Gemini
  (`GEMINI_API_KEY` / `GOOGLE_API_KEY`, defaulting to `gemini-3.7-flash`), and `-backend openai`
  runs on OpenAI (`OPENAI_API_KEY`, defaulting to `gpt-5.4`). Both support streaming, tool
  calling, and reasoning.

## [0.6.1] — 2026-08-28

### Fixed

- **`scripts/check.sh` ran golangci-lint against a toolchain it had not chosen.** The build, vet
  and test passes go through `$GO`, resolved from `GOROOT` so they honour the version this
  repository pins. The lint pass called `golangci-lint` bare, and golangci-lint type-checks with
  whatever `go` it finds on `PATH`. Where a newer one sat there, a Homebrew install ahead of the
  pinned one being the ordinary way, the pass died mid-run on `file requires newer Go version
  go1.27 (application built with go1.26)` under two hundred lines of goroutine dump. It now runs
  under the same toolchain as everything else.

  The failure mode is worse than a broken lint pass, which is why it is worth the paragraph:
  `lefthook` runs this script on `pre-push` and resets the environment, so the gate could not be
  repaired from the calling shell, and the message pointed at nothing the reader could act on. A
  gate that fails for an unactionable reason is a gate that gets pushed past with `--no-verify`,
  which is what happened to 0.6.0.

- **The 0.6.0 note on batching overstated what it saves, and is corrected in place.** It said the
  harness already ran a turn's calls concurrently. That holds on `anthropic`, whose backend hands
  the turn to the SDK's runner; it does not hold on `openrouter`, whose backend runs a turn's
  calls in order. The rule is still worth having on both, because the model round trip is the
  expensive part, but the entry now says which backend does which.

- **The 0.6.0 note on `max_iterations` described the risk without naming the case that carries
  it**, and is likewise corrected in place. The SDK conditions "zero means no cap" on every tool
  being read-only and cheap. `bash: true` mounts a real shell, which is neither, and that is the
  configuration the no-cap default is least safe under.

## [0.6.0] — 2026-08-28

### Added

- **The model is told to batch independent tool calls.** When several calls need nothing from one
  another, it makes them in a single turn rather than one after another, and waits only where a
  later call needs an earlier one's output. What that saves depends on the backend: `anthropic`
  hands a turn's calls to the SDK's own runner, which executes them together, while `openrouter`
  runs them in order. Batching earns its place on both, because the round trip to the model costs
  more than the tools do. The rule lives in the environment preamble rather than the default
  system prompt, because
  `-system` replaces that prompt outright and this is the one piece of working advice that has to
  survive a custom persona.

### Changed

- **`max_iterations` now defaults to `0`, which is no cap.** The old default of 40 ended a long
  run with `Stop: StopIterations` partway through the work, and 40 was never a number chosen for
  a particular task; it was a guess that any genuine multi-step job outgrows. A run now continues
  until the model stops asking for another turn. Put a ceiling back with `max_iterations` in
  `~/.nacelle.yml`, the `-max-iterations` flag or `NACELLE_MAX_ITERATIONS`, all unchanged.

  The trade is worth stating plainly. The SDK conditions "zero means no cap" on every tool being
  read-only and cheap, and `bash: true` mounts `run_command`, which is neither: with no cap, a
  model looping on a failing shell command loops until you interrupt it. Keep a cap where the
  shell is mounted, where the tools are expensive, or where the run is unattended.

### Removed

- **`mycelium: true` in `~/.nacelle.yml` is now a startup error. Delete the key.** The
  `-mycelium` flag, the `NACELLE_MYCELIUM` variable and the `mycelium:` setting are gone, along
  with the `list_flows`, `run_flow` and `search_memory` tools they mounted. This client reads
  `~/.nacelle.yml` with `KnownFields(true)`, which refuses a key it does not recognise rather
  than ignoring it, so a config that still carries the line will not load until the line goes.
  That strictness is deliberate and it is why this is called out here.

  Nothing is lost. `mycelium mcp` serves the same three tools over stdio, so add mycelium's
  server to a file in your `mcp:` list and they come back through the path every other MCP
  server already uses — gated by `-approve-tools` like any other tool, and working on either
  backend. `mycelium install` writes that `mcpServers` entry for you.

  The reason for the change is layering, not the tools. The switch mounted
  `nacelle/tools.Mycelium()`, which made a general-purpose Go SDK depend on one company's
  binary; the SDK dropped the package in the same round.

- Reading mycelium's usage feed is **not** affected. Each finished turn is still appended to
  `~/.mycelium/events/nacelle/`, gated on that directory existing rather than on any setting,
  and the dashboard still shows a live session. That is a separate integration from the tools.

## [0.5.0] — 2026-08-24

### Changed

- A delegate may now use the tools it was given. `-subagents` handed the nested
  run the parent's tool set and then refused every call it made, which is the
  library's default when the caller supplies no policy, so a delegate asked to
  search wide or read a log could do neither and answered from its task
  description alone. It now inherits the parent's own policy: where the parent
  runs every call unasked, so does the delegate, and where `-approve-tools` is
  on, the delegate's calls reach the same prompt. Two things worth knowing
  before turning it on. The prompt names the tool and cannot say that a
  delegate asked for it, and allowing a tool for the session allows it for the
  parent too, because the allow-list is keyed by name.
- `-approve-tools` no longer asks about the plan. The `tasks` tool writes
  nothing outside this process, and everything it says is drawn on screen the
  moment it is recorded, so the question bought nothing and cost a keypress per
  step of every plan.

### Fixed

- The plan no longer outlives the session it belonged to. `/clear` reset the
  conversation, the running total and the reasoning clock but not the steps, so
  a cleared screen opened with the last session's plan still drawn above the
  prompt, and a model with no memory of it never wrote over it. The rows it
  held go back to the live region with it.
- A delegate can no longer replace the plan on screen. The nested run inherits
  the parent's tools minus only the sub-agent tool, so it inherited `tasks` as
  well: a delegate that reported its own steps overwrote the plan the parent
  was working to, and nothing corrected the screen when the delegation ended.
  The tool is now mounted after the delegate has taken its copy, which is where
  anything that draws belongs. Only reachable with `-subagents` on.

## [0.4.3] — 2026-08-24

### Fixed

- A queued message being edited could still be sent. The offset naming it
  counts from the end of the queue, which survives the queue draining from the
  front — but skipping the edited line meant lines behind it were drained too,
  and each of those shortened the queue without moving the edited line closer
  to the end. The offset then pointed past the queue, nothing looked like it
  was being edited, and the next line out was the one still being rewritten.
  The reader's own edit then arrived behind it as a second, near-identical
  message. Found by review, not by use.
- The banner no longer wraps on terminals wider than 80 columns. It is painted
  before any window size has been reported, so it was held to the 80 the model
  starts at and put `· bash on` on a line of its own. What the client says
  about itself is now left for the terminal to wrap, like everything else.
- Session filenames are UTC and carry no punctuation. Local time is not in
  chronological order across a DST boundary or a flight, and sorting the names
  is the only index the directory has; RFC3339 also spells the offset with a
  colon, which Windows and SMB refuse and the Finder renders as a slash.

### Changed

- The reader's own questions carry a background again, not just bold. An
  answer is full of bold — every heading and emphasised phrase the model
  writes — so the one bold line meaning "you said this" was competing with a
  page of them, and scrolling back for your own question meant reading rather
  than glancing.
- The client separates clauses with `·` and no dashes, so the empty-run report
  reads `no answer · nothing billed · try another model`.

## [0.4.2] — 2026-08-24

### Added

- The banner names the client and its version — `nacelle 0.4.2 · openrouter ·
  <model>`. It is the one thing on screen a bug report needs and nothing else
  supplied, on a line that was already there.

### Changed

- The empty-run report is one line and says what to do rather than whose fault
  it is. The first version explained that a model refusing tool definitions is
  the usual cause: true, but it wrapped onto a second row and read as the
  client making an excuse for itself. The diagnosis lives in the source now.

## [0.4.1] — 2026-08-24

### Fixed

- A run that produced nothing now says so instead of returning to `ready`
  under a transcript holding only the question. A stream that yields a turn
  and a done with no text between them is well-formed — nothing errored and
  nothing was refused — so there was no ending to report and the client
  looked like it had ignored the question. When the turn billed no tokens
  either, the report says so: that is a request the provider dropped before
  running it, which is what a model that will not accept tool definitions
  does. Measured against `openrouter/stealth-ox-alpha`, which returns exactly
  this for any request carrying a tool, and answers normally without one.
- The launch banner is visible again without scrolling up. It was handed to
  `tea.Println`, which does not append: it makes room by scrolling the screen
  and inserting above the frame, so on a freshly cleared terminal — where the
  frame stands on the first row — there was nothing above it to insert into
  and the banner went straight into the scrollback. Which backend and model
  are about to be billed now prints before the program takes the screen.
- A queued message pulled into the prompt for editing no longer appears twice.
  It is drawn in the prompt, and was still listed above in the state it was
  being rewritten out of, which read as the edit having failed. It is also no
  longer delivered while it is being edited: a run settling mid-edit used to
  send the wording the reader had already decided was wrong.

## [0.4.0] — 2026-08-24

### Added

- Queued messages can be edited. Up from the prompt now walks the lines still
  waiting to be sent before it walks the questions already sent, and enter puts
  the edited line back where it came from instead of adding a second copy. A
  line that goes out while it is being edited becomes a new message rather than
  overwriting whichever line has taken its place.
- Esc hands the session to the queue. It still stops the answer being written,
  but what was typed behind it now starts immediately instead of being thrown
  away — esc means "not this one, move on", and ctrl+c keeps meaning stop.
- Session transcripts under `~/.nacelle/sessions`, one JSONL file per run.
  Questions, answers, and the name and duration of each tool call. Reasoning,
  tool arguments, tool output and file diffs are never written: the only
  reliable redaction is not collecting it. Directory `0700`, file `0600`.
- A task list the model keeps through a `tasks` tool, drawn under the status
  line, for a job big enough to need splitting into steps.
- Each finished turn is appended to mycelium's event feed when mycelium is
  installed, so a session shows up in its dashboard while it is still running.

### Fixed

- Pressing Up twice with one question in the history crashed the client. The
  walk ran past the start of the list and indexed it at -1.
- Muted text is readable again. Every dimmed style used ANSI 8, which a dark
  scheme is free to park on the background, so "dimmed" came out invisible on
  the terminals that move it. Greys now come from the 256-colour ramp, which is
  not themeable, and follow the terminal's own background.
- The status row says which phase a run is in: the spinner, the phrase and the
  clock share one colour, cyan while the model is being waited on and the
  tool's own colour once something runs. A tool with no glyph of its own — every
  MCP tool — was drawn with no colour at all.
- Truncating a styled line charged a cell for every character of an escape
  sequence, so a coloured status line was cut a dozen cells early and cut
  mid-sequence, leaking the colour into everything printed after it.

## [0.3.0] — 2026-08-24

### Added

- Prompt history: Up recalls sent questions newest-first, Down walks back
  forward and restores the draft being written; from a wrapped question's
  later rows, Up still moves the caret.
- A two-row status bar — what is happening (spinner, phrase, run clock) on
  top, what the session has spent underneath.
- `/cost` says what the session has spent so far, on demand.
- The command menu marks its selected row with an arrow.

### Changed

- The palette is ANSI indices now, so every shade follows the terminal's own
  scheme; the question entry is bold instead of carrying a background. The
  status spinner takes its colour from the phase — dim while thinking,
  tool-blue once something runs.
- The suggestion menu draws below the prompt, which no longer moves the
  input line when the list changes; the prompt gained a gutter marker and a
  blank row of its own, and one empty row follows the launch banner.
- The core SDK is pinned at v0.4.1, up from v0.4.0: sub-agent delegation
  (`NewSubAgentTool`) arrives in core, opt-in here via `subagents:`.

### Added

- File edits draw a git-style diff under the tool's one-line report: removals in the
  terminal's red, additions in its green, with three lines of context around each block and a
  cap of four hundred lines. `edit_file` diffs from its own old/new arguments; `write_file`
  from what the file held when the call was seen against the contents it wrote. Off with
  `diffs: false` (`NACELLE_DIFFS`, `-diffs`), which restores the bare one-line report.

- `-subagents` / `subagents:` / `NACELLE_SUBAGENTS`, off by default. When on, the model
  gets a `subagent` tool that delegates a self-contained task to a fresh nested run — its
  own context window, the same backend and tools — and only the delegate's final answer
  comes back. The nested run cannot delegate further and denies every tool call by
  default; it runs unattended or not at all. A delegate's token spend lands in the
  session's totals as it is spent.

### Fixed

- The gofmt pass in `scripts/check.sh` handed git's file list to `xargs` as
  whitespace-delimited text. Git passes spaces in filenames through unquoted, so a path
  with a space was split and half of it formatted; the list is null-delimited now (`-z`
  feeding `xargs -0`), which also stops GNU `xargs` running gofmt once on an empty list
  with stdin attached.
- The status line's waiting phrase is bucketed from when this run began rather than from
  the wall clock, so every wait opens on the first phrase instead of wherever the epoch
  happened to be.

## [0.2.1] — 2026-08-24

### Changed

- Homebrew installs from a cask rather than a formula. `brew install
  FacileStudio/tap/nacelle` is unchanged; what moves is the tap file, from
  `Formula/nacelle.rb` to `Casks/nacelle.rb`. GoReleaser soft-deprecated `brews` in v2.10 and
  hard-deprecated it in v2.16, and release CI runs 2.17.1, so the previous config would have
  failed its own check on the next tag. A cask download carries the quarantine attribute
  where a formula's did not, so a `postflight` hook strips it — without that, the first run
  of the unsigned binary dies on "the developer cannot be verified".
- The core SDK is pinned at v0.4.0, up from v0.3.1. The only change between the two is the
  split that created this repository, so nothing here had to adapt.

## [0.2.0] — 2026-08-24

### Added

- `ctrl+t` expands a turn's reasoning, and keeps showing it in full until pressed again. It
  costs the prompt its `transpose-character-backward` binding.

### Changed

- A tool call reads as `read_file(view.go) · 12ms` instead of the tool name followed by its
  raw JSON input. The argument is picked by key (`path`, `file_path`, `file`, `command`,
  `pattern`, `query`, `url`, `name`), falling back to the sole key of a one-argument call.
- A tool that succeeds is one line carrying its duration, not a call line plus a
  `done in 12ms` line under it. Failures keep both lines. The call line is held until the
  result arrives, which the status line already covers by naming the running tool; a run that
  ends with calls in flight still prints them, without a duration.
- Reasoning collapses to `· thought for 4.2s`, with the `ctrl+t` hint shown once per session.

### Fixed

- The thinking duration measured the whole turn. A turn is committed at its end, which is
  after the answer has streamed and after any tool the model called has run, so 0.6s of
  reasoning followed by a 0.6s answer printed `thought for 1.2s`. The clock now stops at the
  first answer delta or tool call.
- `/clear` left the retained reasoning behind, so `ctrl+t` reprinted the thinking from the
  session that was just cleared.
- A run that ended mid-tool said the held call line above the sentence that announced it.
- The build binary `nacelle-tui` was tracked rather than ignored; plain `go build` writes it
  and `.gitignore` only listed the old `nacelle` name. Untracked here, though the objects
  already in history stay there.

### Security

- Tool-call input carrying a repeated key is refused rather than summarised.
  `encoding/json` silently keeps the last value, so `{"command":"ls","command":"rm -rf /"}`
  could be shown as `run_command(ls)` over a call that ran something else. With
  `-approve-tools` on, such a call is denied before any prompt is drawn and before the
  session allow-list is consulted, since allow-for-session is permission for a tool granted
  against a legible call, not a standing waiver on whatever input arrives afterwards.

## [0.1.0] — 2026-08-23

### Added

- First tagged release of the terminal client, extracted from
  [FacileStudio/nacelle](https://github.com/FacileStudio/nacelle) where it lived as `tui/`.
  The module is now `github.com/FacileStudio/nacelle-tui` and pins core v0.3.1.
- `-version` flag printing exactly `nacelle <semver>`, stamped by goreleaser's ldflags into
  `main.version`.
- Distribution per CLI-STANDARD §5/§9: GoReleaser archives for darwin/linux amd64+arm64,
  Homebrew formula in `FacileStudio/tap`, an `install.sh` shim, and a `nacelle` entry in the
  `facile` catalog.

[Unreleased]: https://github.com/FacileStudio/nacelle-tui/compare/v0.16.1...HEAD
[0.15.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.15.0
[0.16.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.16.0
[0.16.1]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.16.1
[0.14.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.14.0
[0.12.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.12.0
[0.11.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.11.0
[0.10.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.10.0
[0.9.1]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.9.1
[0.9.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.9.0
[0.8.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.8.0
[0.7.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.7.0
[0.6.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.6.0
[0.5.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.5.0
[0.4.3]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.4.3
[0.4.2]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.4.2
[0.4.1]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.4.1
[0.4.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.4.0
[0.3.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.3.0
[0.2.1]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.2.1
[0.2.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.2.0
[0.1.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.1.0
