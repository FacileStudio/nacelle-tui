# Parallel Sub-Agents — Actual Implementation

This is the implementation doc, not the original plan. There are two delegation paths, and they differ in who is free while the work runs.

- The **detached** path — the `/parallel` command a person types. The TUI launches the fan-out itself, outside any parent run. The main thread stays free, so you keep chatting while the subagents grind. This page describes it.
- The **model-callable** path — `parallel_subagent` as a tool the parent model calls mid-turn. The parent must wait on its tool result, so this path blocks the parent for the fan-out, and a message typed then queues.

## What shipped

- `/parallel task1, task2, task3` — detached user-facing command
- `internal/tui/parallel_cmd.go` — parser, detached fan-out launcher, result watch
- `internal/agent/delegate.go` — wires `NewParallelSubAgentTool` into the tool set (the model-callable path)
- nacelle `DelegateParallel` — the detached fan-out API (subagent_parallel.go)
- `internal/tasks/taskplan.go` — agent-facing task-plan tooling

## How the detached path works

1. User types `/parallel a, b, c`
2. `splitParallelTasks` splits on `,`, drops empty entries
3. `launchDetached` creates a row batch under the prompt and says "started N parallel agents" in the main thread
4. A goroutine calls nacelle `DelegateParallel` with the parent's own `Config` — so each nested agent sees the same tools, system prompt and iteration ceiling it would under the model-callable tool
5. nacelle fans out to N concurrent nested agents, posting each task's result through a channel as it finishes
6. `recordDetached` on the update loop applies each result to its row, folds the task's spend into the session total, and re-lays out
7. The main run's `busy` flag is never touched, so the prompt stays live the whole time; a question typed mid-fan-out starts a normal run

## What the TUI does NOT do

- No `Model.agents` map for the fan-out. No tabbed layout.
- No per-agent event routing for the parallel children. No `AgentID` envelope field (nacelle derives the nested config and streams results through the detached channel).

## How nacelle does the fan-out

- `nacelle.DelegateParallel(ctx, cfg, tasks, opts)` returns a `<-chan nacelle.ParallelTaskResult` the caller drains.
- Each task runs in its own fresh agent on `cfg`, capped at `MaxConcurrency` (default 4, max 8).
- Each `ParallelTaskResult` carries the task's index, result text or error, and its own spend.
- The channel closes when the last task finishes; an empty task list is an error, returned rather than posted.
- `NewParallelSubAgentTool` (the model-callable tool) blocks its caller until all tasks return one merged JSON — the shape the parent model consumes.

## What's NOT implemented (YAGNI)

- Tabbed multi-agent UI
- Agent worktrees, distributed file locks, Redis coordination — not a TUI concern
- `AgentID` envelope

## Files

| File | Role |
|---|---|
| `internal/tui/parallel_cmd.go` | `/parallel` parser + detached launcher + result watch |
| `internal/agent/delegate.go` | wires `NewParallelSubAgentTool` (model-callable path) |
| `internal/agent/agent.go` | `build` returns the `Config` the TUI clones for detached fan-out |
| `internal/tui/parallel_result.go` | model-callable result handling + task rows |
| `internal/tui/parallel_titles.go` | one no-tool summarizer call turning a fan-out into 6-7 word task titles |
| `internal/tasks/taskplan.go` | task-plan tooling |

## Conventions

- `[module-path]`: `github.com/FacileStudio/nacelle-tui`
- `[filet]`: `filet check .` passes; `filet test` passes
- `[events]`: no change — the model-callable path still collapses parallel results into a single tool result; the detached path uses a new `detachedResult` message on the update loop
- `[migrations/auth/muse/distribute]`: N/A
