# Plan: Parallel Sub-Agents (nacelle-tui)

## Goal
The TUI shows and orchestrates multiple parallel sub-agents running concurrently, rather than only one run at a time. Users can split a task with `/parallel task1, task2, task3` and watch each branch complete as it finishes, with a tabbed UI to switch between them.

## Why
The TUI is currently a single-pane experience: the `Model` holds one `run inflight`, and `send()` blocks on the previous run. Users who want to investigate three unrelated files end up serialising what is naturally parallel work.

## Approach
Reuse the existing `AgentRunner` and `inflight` machinery once per agent, indexed by agent ID. The TUI grows a `Model.agents` map field that mirrors the single-run state. A new `/parallel` command is a thin parser that splits the input and launches N agents in `m.agents`. A new `parallel_view.go` renders the tabbed layout.

## Conflict Prevention (for file editing scenarios)

### 1. File ownership boundaries (primary)
Before launching, the `/parallel` command or parent agent decomposes the task so each agent owns a distinct file subset:
- Agent 1 → `apps/api/modules/auth/*.go`
- Agent 2 → `apps/api/modules/users/*.go`
- Agent 3 → `apps/api/migrations/*.sql`

If agents have no overlapping files, there are zero merge conflicts. This is the single most effective practice.

### 2. Git worktrees per agent
Each parallel agent gets its own git worktree at spawn time (implemented in the TUI's agent runner). The agent operates on a frozen copy of the repo state from that moment. The coordinator waits for terminal status, reads the diff, and decides whether to merge. Pattern from the OctoCo AI interview: "Each task agent gets its own git worktree at spawn time, so it's operating on a frozen copy of the repo state from that moment."

### 3. Distributed file locks
If agents must touch overlapping files, use Redis `SET lock:<file> <agent-id> NX EX 300` to acquire exclusive 5-minute locks. If the lock is held, the agent waits and retries. The coordinator owns the lock on shared files/tasks and only writes back when the agent reports terminal status.

### 3. Context snapshotting
Each agent receives a snapshot of the codebase state at spawn time (via system prompt + initial conversation), not a live-updating view. This prevents "context drift" where Agent 7 operates on a stale view that Agents 1-3 already modified. The coordinator reconciles diffs at completion time.

### 4. Plan / Execute split
- **Plan phase (parent)**: Decompose the task into independent subtasks, assign file ownership, create the task list
- **Execute phase (parallel agents)**: Each agent works on its assigned subtask with its own context window
- **Synthesis phase (parent)**: Collect results, detect conflicts, merge or escalate

### 5. Task claiming from shared list
If using a shared task list, agents claim tasks one at a time; each task carries its own file ownership. Task claiming uses distributed locks to prevent two agents claiming the same task.

### 6. Size tasks appropriately
- Too small: coordination overhead exceeds benefit
- Too large: agents work too long without check-ins, increasing wasted effort
- Just right: self-contained units producing a clear deliverable (function, test file, review)

Rule of thumb: 3-6 tasks per parallel agent; narrow scope → small diffs → solvable merge.

## Files to Modify

1. `internal/tui/model.go` — add `agents map[string]*agentSpec`, `activeAgent string`, refactor single-run paths
2. `internal/tui/inflight.go` — convert from a single-run tracker to a per-agent tracker; the `run` field moves into `agentSpec`
3. `internal/tui/run.go` — `send()` fans out for parallel tasks; `consume()` and `absorb()` route per-agent events into the right `agentSpec`
4. `internal/agent/build.go` — wire the new `ParallelSubAgentTool` into the tools list
5. `internal/tui/views/parallel_view.go` (new) — tabbed view: each tab is one agent, shows its reasoning, tool calls, result
6. `internal/tui/commands/parallel_cmd.go` (new) — `/parallel task1, task2, task3` parser and dispatcher
7. `internal/tui/run_test.go` — extend existing tests to cover the multi-agent state machine

## Exit Criteria

### nacelle-tui
- `go build ./...` silent.
- `filet check .` silent.
- All existing tests pass.
- `TestParallelViewRendering`: three agents in the map render as three tabs; switching `activeAgent` updates the visible pane.
- `TestParallelCommandParsing`: `/parallel task1, task2, task3` produces three `agentSpec` entries; `/parallel task1` is a no-op for parallelism but still routes through the same path
- `TestParallelPartialFailure`: one agent fails, the other two return results, the UI marks the failed tab
- `/parallel` end-to-end: launches 3 agents on a real backend, all 3 return their result, the UI shows all three without crashing
- When one parallel agent fails, others continue and return their results; the UI marks the failed tab

## Risks

1. **Event interleaving**: tool calls from one agent and results from another arrive on the same stream. Each event must carry an `AgentID` so `consume()` routes correctly. The existing event contract is already per-call stream, so this needs a new envelope field.
2. **Terminal width**: 3 streams side by side is tight. The tabbed layout is the minimum viable version; accordion (all visible, only the active one scrolls) is the natural next step.
3. **MCP sessions**: each parallel agent shares the MCP bridge. The bridge is already concurrent-safe with per-call IDs.
4. **State in `Model`**: the single `run inflight` field is read in many places. Refactoring it to a map touches every place that reads it. Plan: introduce the map, leave the single field as a view of `m.agents[m.activeAgent]`, deprecate later.
5. **Cancellation**: if the user Ctrl-C's, all in-flight agents stop on parent context cancellation.
6. **Order of completion**: UI shows agents in submission order, not completion order. The status indicator per tab updates as results arrive.

## Skip (YAGNI)

- Do NOT change the `Backend` interface or `Agent` struct
- Do NOT make the model itself run in parallel
- Do NOT remove the existing single-run UI. It stays for non-parallel flows; only the parallel path takes the new code
- Do NOT add rate-limiting or backpressure. Tasks fire immediately, bounded by the configured `MaxConcurrency`
- Do NOT add a "merge results" agent step. The parent sees the raw map; whatever asks for parallelism decides what to do with the results.

## Convention flags

- `[migrations]`: N/A
- `[auth/porte]`: N/A
- `[muse]`: N/A — TUI is Bubble Tea, not Svelte
- `[module-path]`: Go modules stay `github.com/FacileStudio/nacelle-tui`
- `[filet]`: plan the code to pass `filet check .` silently
- `[events]`: parallel agents carry an `AgentID` in the event envelope so the TUI can route per-agent events. The TUI consumer reads it and dispatches to the right `agentSpec`
- `[distribute]`: N/A

## Open Questions

1. **Tabbed vs accordion**: tabbed (one agent visible at a time, switch with `Tab`/`Shift+Tab`) is the minimum. Accordion (all agents visible, collapsed by default, only the active one scrolls) is the natural next step.
2. **Result merging**: when the model itself asked for parallel sub-agents, the parent's `Stream` returns the merged map. The TUI shows each branch in its own tab, then a "synthesis" tab with the parent's final answer. The parent's final answer is just another `agentSpec` for the parent itself.
3. **Order of completion**: UI shows agents in submission order, not completion order. The status indicator per tab updates as results arrive.
4. **Naming**: agent IDs are auto-generated short strings (`agent-1`, `agent-2`, ...) or the task string itself truncated. Auto-generated, with the task shown as the tab label.

## Tools

- `gh` / `git` / `rg` / `find` for recon.
- `filet` to *check* current state, not to plan around it.