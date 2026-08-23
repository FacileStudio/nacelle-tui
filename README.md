# nacelle

A terminal coding agent — the human harness for the
[nacelle](https://github.com/FacileStudio/nacelle) SDK.

The SDK is a Go library for building agents; this program exists to exercise
every part of it from a terminal, where someone is watching: text, reasoning,
tools starting and finishing, why a turn ended, what it cost. It is deliberately
small — sessions, profiles and panes are what a product grows, not what a
contract test needs.

## Install

```sh
brew install FacileStudio/tap/nacelle
```

or with the Facile CLI:

```sh
facile install nacelle
```

## Usage

Run it in the directory you want it to work in:

```sh
nacelle
```

It reads API keys from the environment — `ANTHROPIC_API_KEY` for the default
`anthropic` backend, `OPENROUTER_API_KEY` for `-backend openrouter`.

Settings layer bottom-up: defaults, then `~/.nacelle.yml`, then `NACELLE_*`
environment variables, then flags. The useful ones:

| Flag | Env | What |
|---|---|---|
| `-backend` | `NACELLE_BACKEND` | `anthropic` or `openrouter` |
| `-model` | `NACELLE_MODEL` | model id, backend's own default otherwise |
| `-root` | `NACELLE_ROOT` | directory the file tools may reach |
| `-bash` | `NACELLE_BASH` | let the model run commands (off by default) |
| `-approve-tools` | `NACELLE_APPROVE_TOOLS` | ask before every tool call |
| `-max-iterations` | `NACELLE_MAX_ITERATIONS` | turn ceiling |
| `-mcp` | — | MCP servers file (repeatable), e.g. `-mcp ~/.claude/.mcp.json` |
| `-skill-dir` | `NACELLE_SKILL_DIRS` | extra skills directory (repeatable) |
| `-mycelium` | `NACELLE_MYCELIUM` | let the model use mycelium flows and memory |
| `-version` | — | print the version and exit |

See `-h` for the full set, including reasoning effort, web search/fetch and
project-context discovery.

## Development

```sh
sh scripts/check.sh            # gofmt + build + vet + test -race + lint
```

Requires Go 1.26.4 (Bubble Tea v2's floor).
