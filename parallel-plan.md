# Parallel Agents Implementation Plan

## Goal
Enable nacelle and nacelle-tui to support multiple parallel sub-agents (subagents running concurrently rather than sequentially), so the user can split work across independent agent processes and collect results as they finish.

## Why (evidence)
The current `NewSubAgentTool` in nacelle creates a single nested agent that must finish before the parent can proceed. The TUI's `inflight` struct and `Model` also only support one run at a time (`run` field, single `started` channel). Users wanting to run three independent searches in parallel must either chain them sequentially or manually manage separate nacelle processes. Parallel agents are the missing piece that would let the model split work naturally — e.g., "search events in repo A, search docs in repo B, analyze this codebase" — all at once.

## Approach
Extend the existing subagent infrastructure rather than creating a separate mechanism:

1. **In nacelle core**: Modify `NewSubAgentTool` / add a `ParallelSubAgentTool` that fans out to N concurrent agents, each running independently on the same backend. Results are collected via a channel and returned as a merged map[string]string.

2. **In nacelle-tui**: Update `Model` and `inflight` to support multiple concurrent runs; add a `/parallel` command that lets the user specify multiple tasks for parallel execution; update the UI to display results per-agent in a tabbed/accordion layout.

3. **Shared patterns**: Both packages already support concurrent tool calls (the model can ask for two tools in one turn) and concurrent `Approve` (terminal serialises prompts). The new parallel-agent layer reuses these primitives.

## Files to Modify / New

### nacelle/core changes:
- **`subagent.go`**: Add `ParallelSubAgentOptions` struct with `Tasks []string` field; modify `NewSubAgentTool` or add `NewParallelSubAgentTool` that spawns N agents and collects results via a done channel.
- The existing `delegate` function model stays; the new tool returns a map of task → result rather than a single final text.

### nacelle-tui changes:
- **`internal/tui/model.go`**: Add `agents map[string]*agentRunner` field; replace single `run inflight` with a mechanism to track multiple concurrent agents; add `activeAgent` to track which one has focus.
- **`internal/tui/inflight.go`**: Refactor from a single-run tracker to a per-agent tracker (map[string]inflight).
- **`internal/tui/run.go`**: Update `send()` to fan out to parallel agents when `/parallel` is used; update `consume()` / `absorb()` to handle per-agent event routing.
- **`internal/tui/delegate.go`**: Add `/parallel` command handler or extend existing delegation.
- **`internal/agent/build.go`**: Wire the new parallel tool into the tools list.

### New files:
- **`internal/tui/views/parallel_view.go`**: Tabbed/accordion view showing each parallel agent's stream, results, and status.
- **`internal/tui/commands/parallel_cmd.go`**: `/parallel` command parsing and dispatch.

## Steps (ordered)

1. **`nacelle/subagent.go`** — Add `ParallelSubAgentOptions.Tasks []string` and `NewParallelSubAgentTool` that:
   - Spawns N goroutines, each running `agent.Stream()` with its own task from `Tasks`
   - Collects results on a `chan map[string]string` that closes when all agents finish
   - Returns a `Tool` whose `Run()` feeds the task JSON and waits for the result map

2. **`nacelle/subagent_test.go`** — Add `TestParallelSubAgentReturnsAllResults` that:
   - Creates a pool of 3 agents with distinct tasks
   - Verifies all 3 results are returned and no task is lost
   - Verifies partial failure still returns completed tasks

3. **`internal/tui/model.go`** — Add `agents map[string]*agentSpec` where `agentSpec` holds the agent, its inflight state, its task, and its results channel; keep `activeAgent` string for UI focus.

4. **`internal/tui/inflight.go`** — Refactor: remove the single `run` field from `Model`; instead `m.agents["agent-1"] = &inflight{...}` etc. Update `start()`, `waitFor()`, `consume()` to dispatch per-agent.

5. **`internal/tui/run.go`** — Update `send()` to, when the model detects a `/parallel` task, split the prompt into N tasks and launch each as a separate agent in `m.agents`; the first agent shown in the UI is the `activeAgent`.

6. **`internal/tui/views/parallel_view.go`** (new) — Tabbed view: each tab = one agent; shows its reasoning, tool calls, result; collapsed view shows agent name + first result line.

7. **`internal/tui/commands/parallel_cmd.go`** (new) — `/parallel task1, task2, task3` → sends to the agent orchestrator which fans them out.

8. **`internal/agent/build.go`** — Add the new `ParallelSubAgentTool` to the tools list when config asks for it.

## Files to Modify (convention flags)

- `[migrations]`: N/A — no database changes
- `[auth/porte]`: N/A — no auth changes
- `[muse]`: UI work in `parallel_view.go` uses muse tokens (Svelte 5 runes) and mobile-first layout
- `[module-path]`: N/A — Go module unchanged
- `[filet]`: Ensure all new code passes `filet check .` silently; plan the code so it passes
- `[events]`: Parallel agents still emit `KindToolCall`/`KindToolResult` per the existing event contract
- `[distribute]`: N/A — no cross-repo deps

## Exit criteria
- `go build ./...` silent across nacelle and nacelle-tui
- `filet check .` silent
- All existing tests pass (no regression)
- New test `TestParallelSubAgentReturnsAllResults` passes in nacelle
- `/parallel` command in nacelle-tui can spawn 3 agents, each returns its result, and the UI shows all three without crashing
- When one parallel agent fails, the others continue and return their results; the UI marks the failed one

## Risks / unknown unknowns
- The `Agent.Stream` model expects one conversation per run; spawning N agents means N separate conversation trees. Merging results without losing context is the main tricky part.
- Terminal width: showing 3 agent streams side by side may be too tight. The tabbed layout is the fallback.
- If the model asks for tools while parallel agents are running, tool calls may interleave with agent results. The existing concurrent-Tool.Run safeguard should cover this.
- MCP bridges: each parallel agent gets its own MCP connection if needed; need to ensure we don't exhaust file descriptors or session limits.

## Skip (YAGNI)
- Do NOT change the `Backend` interface or the `Agent` struct — those are shared across all consumers.
- Do NOT make the model itself run in parallel (the model is single-threaded; only the agent loops are parallel).
- Do NOT remove the existing single-subagent `/subagent` tool — keep it working as before; the parallel variant is an addition.

## Convention flags applied inline

1. Step 1 (`subagent.go`): `[migrations]` not applicable; tool schema follows the existing `subAgentInput` pattern with `jsonschema:"required"` on `task`.
2. Step 3 (`model.go`): `[module-path]` — Go module `github.com/FacileStudio/nacelle-tui` stays; never fork-inherit bare module.
3. Step 5 (`run.go`): `[muse]` — UI layout uses muse design tokens for padding and color; mobile-first means tabs stack vertically on narrow terminals.
4. Step 7 (`parallel_cmd.go`): `[events]` — each agent's tool calls and results follow the same event contract; the parent just fans the channels.
5. All steps: `[filet]` — the plan says code should pass filet; we will verify after implementation.

## Pause for review

Before anything executes, confirm the plan direction:

1. Should we start with the nacelle core changes (steps 1-2) or the TUI changes (steps 3-8)?
2. Tabbed view vs accordion vs split-pane for the parallel-agent UI — which does the user prefer?
3. Should `/parallel` be a new command or should we extend the existing `/subagent` with a `--parallel` flag?

A 60-second checkpoint here catches a wrong direction cheaply. Say yes/no/adjust per each item.

## Tools
- `gh` / `git` / `rg` / `find` for recon (read-only).
- `filet` only to *check* current state after changes, not to plan around it.