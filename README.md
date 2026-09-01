# nacelle-tui

Terminal coding agent — the human harness for the
[nacelle](https://github.com/FacileStudio/nacelle) agent SDK.

The binary is named `nacelle`. The SDK is a Go library for building agents;
this program is its first consumer and lives to exercise every part of it from
a terminal, where someone is watching: text, reasoning, tools starting and
finishing, why a turn ended, what it cost. It is deliberately small — sessions,
profiles and panes are what a product grows, not what a contract test needs.

## What it does

- Streams one model turn at a time in a full-screen Bubble Tea v2 interface
- Runs against any backend the SDK ships: `anthropic`, `google`, `openai`, or `openrouter`
- Lets the model read and edit files under a root you choose, run commands when
  `-bash` is on, search and fetch the web, and call MCP server tools from files
  every other client already has (`-mcp ~/.claude/.mcp.json`)
- Lets the model lay a large job out as steps and keep them current while it
  works, drawn live above the prompt and scrolled to the step in flight
- Delegates a self-contained side task to a nested run with `-subagents`, so a
  wide search or a log dump costs the conversation one answer instead of its
  whole output
- Discovers project context (CLAUDE.md, AGENTS.md) and skills into the system
  prompt, each behind its own flag
- Gates tool calls behind an approval prompt with `-approve-tools`, and trusts
  project hook files only after an explicit, remembered decision

## Stack

| Layer | Tech |
|---|---|
| TUI | Go 1.26.4, `charm.land/bubbletea/v2`, lipgloss v2, glamour v2 |
| Agent | [FacileStudio/nacelle](https://github.com/FacileStudio/nacelle), pinned by tag |
| State | `~/.nacelle.yml`, `~/.nacelle/trust.json` for hook trust |
| Release | GoReleaser, GitHub Actions on tag push, Homebrew tap `FacileStudio/tap` |

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/FacileStudio/nacelle-tui/main/install.sh | bash
```

Installs to `~/.local/bin` via [facile](https://github.com/FacileStudio/facile), the suite
installer. Pass `--bin-dir <dir>` to change that, `--source` to build from source.

Already have `facile`:

```sh
facile install nacelle
```

Or Homebrew:

```sh
brew install FacileStudio/tap/nacelle
```

## Usage

Run it in the directory you want it to work in:

```sh
nacelle
```

It reads API keys from the environment: `ANTHROPIC_API_KEY` for the default
backend, `GEMINI_API_KEY` (or `GOOGLE_API_KEY`) for `-backend google`,
`OPENAI_API_KEY` for `-backend openai`, and `OPENROUTER_API_KEY` for `-backend openrouter`.

Settings layer bottom-up: defaults, then `~/.nacelle.yml`, then `NACELLE_*`
environment variables, then flags. The useful ones:

| Flag | Env | What |
|---|---|---|
| `-backend` | `NACELLE_BACKEND` | `anthropic`, `google`, `openai`, or `openrouter` |
| `-model` | `NACELLE_MODEL` | model id; the backend's own default otherwise |
| `-root` | `NACELLE_ROOT` | directory the file tools may reach |
| `-bash` | `NACELLE_BASH` | let the model run commands (off by default) |
| `-approve-tools` | `NACELLE_APPROVE_TOOLS` | ask before every tool call runs |
| `-subagents` | `NACELLE_SUBAGENTS` | give the model a delegate with its own context window (off by default) |
| `-max-iterations` | `NACELLE_MAX_ITERATIONS` | how many times the model may be asked |
| `-mcp` | — | MCP servers file (repeatable) |
| `-skill-dir` | `NACELLE_SKILL_DIRS` | extra skills directory (repeatable) |

`nacelle -version` prints exactly `nacelle <semver>`. See `-h` for the full set:
reasoning effort and budget, web search/fetch, project-context and skill
discovery, hooks trust.

Full settings reference: [docs/configuration.md](docs/configuration.md).

## Configuration

Settings live in `~/.nacelle.yml`, created with defaults the first time a
setting needs writing. Hook trust decisions live in `~/.nacelle/trust.json`,
keyed by project path and file hash — trusting a hook file is remembered
across runs, per project, never globally.

## Structure

```
main.go        Entrypoint: build config, tools, hooks and the agent, then launch
flags.go       Command line layer, one flag per Config field
config.go      Defaults and the ~/.nacelle.yml layer
env.go         The NACELLE_* environment layer
conversation.go, command.go, compact.go   Transcript, slash commands, compaction
hooks*.go      Project hook files: parsing, trust decisions, process wiring
skills.go      Skill discovery and rendering into the system prompt
```

---

Part of the [Facile Suite](https://facile.studio) — self-hosted tools for creative studios
and freelancers. One login, zero cloud dependency.
