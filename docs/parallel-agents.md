# Parallel Sub-Agents — Actual Implementation

This is the implementation doc, not the original plan. The shipped design differs from what was proposed: the parallel fan-out lives in nacelle, not in the TUI. The TUI's job is just to surface it.

## What shipped

- `/parallel task1, task2, task3` — user-facing command
- `internal/tui/parallel_cmd.go` — parser + prompt builder
- `internal/agent/delegate.go:18` — `withSubagents` wires `NewParallelSubAgentTool` into the tool set
- `internal/tasks/taskplan.go` — agent-facing task-plan tooling

## What the TUI does NOT do

- No `Model.agents` map. No `activeAgent`.
- No `parallel_view.go`. No tabbed layout.
- No per-agent event routing. No `AgentID` envelope field.
- The single `Model.run inflight` is unchanged.

## How it works

1. User types `/parallel a, b, c`
2. `splitParallelTasks` splits on `,`, drops empty entries
3. `buildParallelPrompt` builds a user message naming the `parallel_subagent` tool
4. `m.send(prompt)` routes the message through the normal single-run path
5. The parent agent calls `parallel_subagent` with a list of tasks
6. nacelle fans out to N concurrent nested agents internally
7. The parent stream receives one merged JSON result: `{"tasks":{"0":"...","1":"..."},"errors":{...}}`
8. The TUI renders the parent's final message as ordinary conversation

## How nacelle does the fan-out

- `nacelle.NewParallelSubAgentTool` takes `MaxConcurrency`
- Each task runs in its own fresh agent on the same backend
- Results are collected into a JSON map by task index
- Errors per task appear in a separate `errors` field
- Nested runs' thinking and usage are consumed at the tool level — they do not surface in the parent's event stream

## What's NOT implemented (YAGNI)

- Tabbed multi-agent UI — parent agent sees merged result, TUI shows one conversation
- Per-agent event routing — nacelle collapses the fan-out to a single tool result
- Agent worktrees, distributed file locks, Redis coordination — not a TUI concern
- `AgentID` envelope — nacelle's tool result is one event to the TUI

## Files

| File | Role |
|---|---|
| `internal/tui/parallel_cmd.go` | `/parallel` parser + prompt |
| `internal/agent/delegate.go:18` | wires `NewParallelSubAgentTool` |
| `internal/tasks/taskplan.go` | task-plan tooling |

## Conventions

- `[module-path]`: `github.com/FacileStudio/nacelle-tui`
- `[filet]`: `filet check .` passes; `filet test` passes
- `[events]`: no change — nacelle collapses parallel results into a single tool result, no new envelope fields
- `[migrations/auth/muse/distribute]`: N/A
