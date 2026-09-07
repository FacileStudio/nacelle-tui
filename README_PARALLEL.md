# Parallel Agents Implementation

## Summary

This implementation adds support for running parallel agents in the nacelle-tui terminal client.

## What Changed

### Core Changes (nacelle library)
1. **Added ParallelSubAgentTool** in `nacelle/subagent_parallel.go`
   - New tool type: `parallel_subagent`
   - Takes a list of tasks (e.g., `["task1", "task2", "task3"]`)
   - Runs each task in its own agent concurrently
   - Returns results as JSON: `{"tasks": {"0": "result1", "1": "result2"}}`

2. **Updated delegate wiring** in `internal/agent/delegate.go`
   - `withSubagents()` now mounts BOTH regular subagent AND parallel subagent tools
   - Both tools use the same approver policy

### TUI Changes
1. **Added `/parallel` command** in `internal/tui/command.go`
   - User can type: `/parallel task1, task2, task3`
   - Calls the `parallel_subagent` tool
   - Shows progress in the UI

2. **UI Display Updates**
   - Parallel agent results show in the conversation transcript
   - Results appear as they complete (order may vary)
   - Failed tasks are marked with errors

## How to Use

1. Start the TUI: `nacelle-tui`
2. Type `/parallel` followed by tasks separated by commas:
   ```
   /parallel analyze this codebase, search for TODO comments, summarize the findings
   ```

3. Watch as multiple agents work in parallel:
   - Results appear as they complete
   - Each result shows which task it answered
   - Failed tasks show error messages

## Key Features

- **True Parallelism**: Multiple agents run simultaneously
- **Independent Context**: Each task has its own agent instance
- **Flexible Output**: Results collected and displayed as they arrive
- **Error Resilience**: One task failure doesn't stop others
- **Backwards Compatible**: Existing single-task `/subagent` tool still works

## Files Modified

1. `internal/agent/delegate.go` - Added parallel subagent tool
2. `internal/tui/command.go` - Added `/parallel` command
3. `internal/tui/run.go` - Updated model config passing

## Testing

Run tests to ensure no regressions:
```bash
go test ./...
```

## Notes

- The parallel implementation reuses the existing subagent infrastructure
- Each parallel task gets its own agent instance with the same backend
- Results are merged and returned to the user in a structured format
- The TUI doesn't need to manage individual agent streams - the tool handles that
