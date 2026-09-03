# nacelle-tui Roadmap

This file tracks UI-only work. Core SDK changes live in `../nacelle/ROADMAP.md`. Both repos release with the same tag (e.g. `v0.8.0` / `tui/v0.8.0`) in a two-commit flow: core first, then UI pinned to it.

---

## Track D — Legibility (UI only)

1. **Tool-call line summarizer & aggregation** — `view.go` `absorb` replaces raw-input `say` with name + primary argument (`command`, `path`, `pattern`, `query`), truncates via `truncate`, and aggregates consecutive calls. *Done: kind-based grouping in `toolgroup.go`, batch duration and deduplication across failures in v0.21.6/v0.21.7.*
2. **Silent successes** — successful tool result prints nothing; duration folds into call line. Failures keep loud line; repeated identical failures collapse to one line with count. *Done via `failureCollapse`.*
3. **Refuse malformed tool-call JSON** — duplicate keys at trust boundary surface as refused call with reason, not decoder-last-wins. *Done via `strictObject` in `toolline.go`.*
4. **Collapsed thinking** — `thinking.go`: `KindThinking` renders one dim line while running, then `· thought for Ns (ctrl+t to read)`; ctrl+t toggles viewport over `m.run.reasoning`. *Done.*

---

## Track E — Claude-Code-grade Organisation (builds on D)

5. **Batching by kind** — consecutive same-kind calls render as one updating line (`⏺ 4 commands · mycelium sync · ls docs · …`), flushed on kind change or `settle`. Clamped within pane width to avoid live region wrapping. *Done in `toolgroup.go`.*
6. **Status footer** — elapsed timer, input/output/cached tokens, ctx, trimmed, and cumulative cost on status line. *Done via `statusrender.go` footer and `/status` command.*
7. **Turn boundaries** — spacing plus running totals at `KindTurn`. *Done via `turn.go` / `view.go` in v0.21.7.*
8. **Recap before quit** — tools run, tokens, cost, two lines printed after `Run` returns beside exit-transcript dump. *Done via `recap.go`.*

---

## Track H — Sessions, Then Compaction

- **Log rotation & write-failure warning** — rotate session files at 256 KB, gzip old file (`.gz`), start new timestamped file. *Done via `sessionrotate.go`.*
- **Session summary command** — *Done via `/status` command.*
- **Dynamic compaction window** — automatically scales `compactAt` to 75% of `Capabilities.ContextWindow` when unconfigured. *Done in v0.21.7.*
- **Resume** — `--continue` picks the newest session under `~/.nacelle/sessions/<project>/`; `/resume` picker in the TUI to resume past conversation. *Done via `--continue` flag and `/resume` command.*
- **Subagents overview** — show list of running subagents and current task progress one per line under the input prompt (like pi or antigravity).

---

## Release process

1. Core lands changes, tags `vX.Y.Z`.
2. UI bumps `go.mod` to require that core version, applies UI changes, tags `tui/vX.Y.Z`.
3. Both CI pipelines must pass (`go test ./... -race`, `golangci-lint`, `goreleaser`).