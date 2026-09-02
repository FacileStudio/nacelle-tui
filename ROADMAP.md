# nacelle-tui Roadmap

This file tracks UI-only work. Core SDK changes live in `../nacelle/ROADMAP.md`. Both repos release with the same tag (e.g. `v0.8.0` / `tui/v0.8.0`) in a two-commit flow: core first, then UI pinned to it.

---

## Track D — Legibility (current, UI only)

1. **Tool-call line summarizer & aggregation** — `view.go` `absorb` replaces raw-input `say` with name + primary argument (`command`, `path`, `pattern`, `query`), truncates via `truncate`, and aggregates consecutive identical calls showing a count (e.g. `⏺ 3 ls`). *Partially done: grouping exists for identical tool+input (`×N`), needs broader "kind" batching.*
2. **Silent successes** — successful tool result prints nothing; duration folds into call line. Failures keep loud line; repeated identical failures collapse to one line with count. *Done via `failureCollapse`.*
3. **Refuse malformed tool-call JSON** — duplicate keys at trust boundary surface as refused call with reason, not decoder-last-wins. *Done via `strictObject` in `toolline.go`.*
4. **Collapsed thinking** — `thinking.go`: `KindThinking` renders one dim line while running, then `· thought for Ns (ctrl+t to read)`; ctrl+t toggles viewport over `m.run.reasoning`. *Done.*

**Exit:** transcript tests assert first-printed-line ordering; no raw JSON reaches any capture.

---

## Track E — Claude-Code-grade Organisation (builds on D)

5. **Batching by kind** — consecutive same-kind calls (e.g. all `command`, all `read_file`) render as one updating line (`⏺ 4 commands · mycelium sync · ls docs · …`), flushed on kind change or `settle`. Must grow within frame by re-render, never `tea.Println`. Test at small pane sizes (80×14).
6. **Status footer** — elapsed timer and cumulative cost on existing status line (usage tracked in `m.run.usage`). Verb variety in `spin.go`. *Partially done: elapsed timer exists.*
7. **Turn boundaries** — spacing plus running totals at `KindTurn`.
8. **Recap before quit** — tools run, tokens, cost, two lines printed after `Run` returns beside exit-transcript dump. *Done via `recap.go`.*

**Exit:** before/after captures of a realistic session at 80×24; measured line-count drop.

---

## Track H — Sessions, Then Compaction

- **Log rotation & write-failure warning** — rotate session files at 256 KB, gzip old file (`.gz`), start new timestamped file. Surface `⚠️ could not write to session log` in UI when `sessionLog.write` returns `false`.
- **Session summary command** — `nacelle status --summary` prints total questions, answers, tool calls, elapsed time, log size.

---

## Release process

1. Core lands changes, tags `vX.Y.Z`.
2. UI bumps `go.mod` to require that core version, applies UI changes, tags `tui/vX.Y.Z`.
3. Both CI pipelines must pass (`go test ./... -race`, `golangci-lint`, `goreleaser`).